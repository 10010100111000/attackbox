package gateway

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"attackbox/internal/auth"
)

var validUsernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

// ProviderProfile 统一标准化第三方身份
type ProviderProfile struct {
	Provider          string `json:"provider"`
	UID               string `json:"uid"`
	Email             string `json:"email"`
	SuggestedUsername string `json:"suggested_username"`
}

// RegistrationTicket 两步注册暂存凭据
type RegistrationTicket struct {
	Provider          string `json:"provider"`
	UID               string `json:"uid"`
	Email             string `json:"email"`
	SuggestedUsername string `json:"suggested_username"`
	ExpiresAt         int64  `json:"expires_at"`
}

// ---------- 1. 公开端点：获取可用第三方提供商列表 ----------

func (g *Gateway) apiPublicOAuthProviders(w http.ResponseWriter, r *http.Request) {
	configs, err := g.authStore.ListOAuthConfigs()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	var active []string
	for _, c := range configs {
		if c.Enabled && c.ClientID != "" && c.EncryptedSecret != "" {
			active = append(active, c.Provider)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"providers": active,
	})
}

// ---------- 2. 授权重定向端点：GET /api/oauth/{provider}/authorize ----------

func (g *Gateway) apiOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(r.PathValue("provider"))
	if provider != "github" && provider != "outlook" {
		http.Error(w, "unsupported oauth provider", http.StatusBadRequest)
		return
	}

	cfg, err := g.authStore.GetOAuthConfig(provider)
	if err != nil || cfg == nil || !cfg.Enabled || cfg.ClientID == "" {
		http.Error(w, "oauth provider not configured or disabled", http.StatusForbidden)
		return
	}

	// 动态构造该主机上的回调 URL
	scheme := "https"
	if r.TLS == nil && !g.cfg.TrustProxy {
		scheme = "http"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	redirectURI := fmt.Sprintf("%s://%s/api/oauth/%s/callback", scheme, r.Host, provider)

	// 生成带时间戳与防篡改 HMAC 签名的 state
	state, sig := generateOAuthState()
	http.SetCookie(w, &http.Cookie{
		Name:     "attackbox_oauth_state",
		Value:    state + "." + sig,
		Path:     "/api/oauth/",
		HttpOnly: true,
		Secure:   scheme == "https",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300, // 5 分钟有效
	})

	var authURL string
	switch provider {
	case "github":
		q := url.Values{}
		q.Set("client_id", cfg.ClientID)
		q.Set("redirect_uri", redirectURI)
		q.Set("scope", "read:user user:email")
		q.Set("state", state)
		authURL = "https://github.com/login/oauth/authorize?" + q.Encode()
	case "outlook":
		q := url.Values{}
		q.Set("client_id", cfg.ClientID)
		q.Set("response_type", "code")
		q.Set("redirect_uri", redirectURI)
		q.Set("response_mode", "query")
		q.Set("scope", "openid profile email User.Read")
		q.Set("state", state)
		authURL = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize?" + q.Encode()
	}

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// ---------- 3. 回调处理端点：GET /api/oauth/{provider}/callback ----------

func (g *Gateway) apiOAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(r.PathValue("provider"))
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("授权已被拒绝或参数缺失"), http.StatusSeeOther)
		return
	}

	// 1. 严格防 CSRF 验签 State Cookie
	cookie, err := r.Cookie("attackbox_oauth_state")
	if err != nil || !verifyOAuthState(cookie.Value, state) {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("安全凭据校验失败或已过期 (CSRF)"), http.StatusSeeOther)
		return
	}
	// 清理 state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "attackbox_oauth_state",
		Value:    "",
		Path:     "/api/oauth/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// 2. 读取密文并瞬时解密 Client Secret
	cfg, err := g.authStore.GetOAuthConfig(provider)
	if err != nil || cfg == nil || !cfg.Enabled {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("第三方平台未启用"), http.StatusSeeOther)
		return
	}

	masterKey := auth.DeriveMasterKey()
	clientSecret, err := auth.DecryptSecret(cfg.EncryptedSecret, cfg.Nonce, masterKey)
	if err != nil {
		logAuth("decrypt oauth secret for %s failed: %v", provider, err)
		http.Redirect(w, r, "/login?error="+url.QueryEscape("解密第三方凭据失败"), http.StatusSeeOther)
		return
	}

	scheme := "https"
	if r.TLS == nil && !g.cfg.TrustProxy {
		scheme = "http"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	redirectURI := fmt.Sprintf("%s://%s/api/oauth/%s/callback", scheme, r.Host, provider)

	// 3. 用 Authorization Code 换取 Access Token 并拉取 Profile
	var profile *ProviderProfile
	switch provider {
	case "github":
		profile, err = exchangeGitHubProfile(code, cfg.ClientID, clientSecret, redirectURI)
	case "outlook":
		profile, err = exchangeMicrosoftProfile(code, cfg.ClientID, clientSecret, redirectURI)
	default:
		err = errors.New("unsupported provider")
	}

	if err != nil {
		logAuth("oauth profile exchange failed for %s: %v", provider, err)
		http.Redirect(w, r, "/login?error="+url.QueryEscape("换取第三方授权身份失败"), http.StatusSeeOther)
		return
	}

	// 4. 查询是否已绑定现有用户
	existingUser, err := g.authStore.GetUserByOAuth(profile.Provider, profile.UID)
	if err == nil && existingUser != nil {
		// 已绑定：直接签发登录 Session
		tok := randHex(32)
		expires := time.Now().Add(24 * time.Hour)
		g.mu.Lock()
		g.sess[tok] = &session{
			User:    existingUser.Username,
			Role:    existingUser.Role,
			Expires: expires,
		}
		g.mu.Unlock()
		_ = g.authStore.SaveSession(tok, existingUser.Username, existingUser.Role, expires)

		http.SetCookie(w, &http.Cookie{
			Name:     "ab",
			Value:    tok,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   true,
			Expires:  expires,
		})

		g.recordAudit("OAUTH_LOGIN", existingUser.Username, "auth", g.getClientIP(r),
			fmt.Sprintf("第三方快捷登录通过: provider=%s uid=%s", profile.Provider, profile.UID))

		http.Redirect(w, r, "/workspace", http.StatusSeeOther)
		return
	}

	// 5. 首次授权使用：生成 Registration Ticket 供前端弹窗完成两步注册
	ticketStr, err := createRegistrationTicket(profile)
	if err != nil {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("生成临时注册凭据失败"), http.StatusSeeOther)
		return
	}

	// 302 重定向到前端登录页面并携带 ticket 参数
	http.Redirect(w, r, "/login?oauth_ticket="+url.QueryEscape(ticketStr), http.StatusSeeOther)
}

// ---------- 4. 两步注册完成端点：POST /api/oauth/complete-registration ----------

type completeOAuthReq struct {
	Ticket   string `json:"ticket"`
	Username string `json:"username"`
	Password string `json:"password"` // 可选
}

func (g *Gateway) apiOAuthCompleteRegistration(w http.ResponseWriter, r *http.Request) {
	// 系统是否允许自主注册检查
	if settings, err := g.authStore.GetAllSettings(); err == nil {
		if allow, ok := settings["allow_register"]; ok && allow == "false" {
			http.Error(w, `{"error":"管理员已关闭自主注册功能","success":false}`, http.StatusForbidden)
			return
		}
	}

	var req completeOAuthReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
		return
	}

	ticket, err := parseRegistrationTicket(req.Ticket)
	if err != nil {
		http.Error(w, `{"error":"注册凭据无效或已过期，请重新点击第三方授权"}`, http.StatusBadRequest)
		return
	}

	targetUsername := strings.TrimSpace(req.Username)
	if !validUsernameRe.MatchString(targetUsername) {
		http.Error(w, `{"error":"用户名必须为3-32位字母、数字、下划线或连字符"}`, http.StatusBadRequest)
		return
	}

	if len(req.Password) < 6 {
		http.Error(w, `{"error":"请设置至少6位的本地登录密码"}`, http.StatusBadRequest)
		return
	}

	// 事务化创建用户并写入绑定记录
	user, err := g.authStore.CreateUserWithOAuthBinding(targetUsername, req.Password, ticket.Provider, ticket.UID, ticket.Email)
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			http.Error(w, `{"error":"该用户名已被注册，请尝试更换其他用户名"}`, http.StatusConflict)
			return
		}
		http.Error(w, fmt.Sprintf(`{"error":"创建用户失败: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// 创建并写入 Session
	tok := randHex(32)
	expires := time.Now().Add(24 * time.Hour)
	g.mu.Lock()
	g.sess[tok] = &session{
		User:    user.Username,
		Role:    user.Role,
		Expires: expires,
	}
	g.mu.Unlock()
	_ = g.authStore.SaveSession(tok, user.Username, user.Role, expires)

	http.SetCookie(w, &http.Cookie{
		Name:     "ab",
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		Expires:  expires,
	})

	g.recordAudit("OAUTH_REGISTER", user.Username, "auth", g.getClientIP(r),
		fmt.Sprintf("第三方两步注册成功并建立绑定: provider=%s uid=%s email=%s", ticket.Provider, ticket.UID, ticket.Email))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user": map[string]interface{}{
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

// ---------- 5. 管理端 API：GET / POST /api/admin/oauth/settings ----------

type adminOAuthConfigView struct {
	Provider   string `json:"provider"`
	ClientID   string `json:"client_id"`
	Configured bool   `json:"configured"`
	Enabled    bool   `json:"enabled"`
	UpdatedAt  string `json:"updated_at"`
}

func (g *Gateway) apiAdminOAuthGetSettings(w http.ResponseWriter, r *http.Request) {
	configs, err := g.authStore.ListOAuthConfigs()
	if err != nil {
		http.Error(w, `{"error":"failed to load oauth configs"}`, http.StatusInternalServerError)
		return
	}

	configMap := make(map[string]*adminOAuthConfigView)
	for _, c := range configs {
		configMap[c.Provider] = &adminOAuthConfigView{
			Provider:   c.Provider,
			ClientID:   c.ClientID,
			Configured: c.EncryptedSecret != "" && c.ClientID != "",
			Enabled:    c.Enabled,
			UpdatedAt:  c.UpdatedAt.Format(time.RFC3339),
		}
	}

	// 保证 github 和 outlook 均有对应视图返回
	for _, p := range []string{"github", "outlook"} {
		if _, ok := configMap[p]; !ok {
			configMap[p] = &adminOAuthConfigView{
				Provider:   p,
				ClientID:   "",
				Configured: false,
				Enabled:    false,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"configs": configMap,
	})
}

type adminOAuthUpdateReq struct {
	Provider     string `json:"provider"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` // 若为空且已配置，则保留现有密钥
	Enabled      bool   `json:"enabled"`
}

func (g *Gateway) apiAdminOAuthUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req adminOAuthUpdateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request payload"}`, http.StatusBadRequest)
		return
	}

	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	if req.Provider != "github" && req.Provider != "outlook" {
		http.Error(w, `{"error":"invalid provider"}`, http.StatusBadRequest)
		return
	}

	var ctB64, nonceB64 string
	if req.ClientSecret != "" {
		masterKey := auth.DeriveMasterKey()
		var err error
		ctB64, nonceB64, err = auth.EncryptSecret(req.ClientSecret, masterKey)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"加密 Secret 失败: %v"}`, err), http.StatusInternalServerError)
			return
		}
	}

	err := g.authStore.SaveOAuthConfig(req.Provider, strings.TrimSpace(req.ClientID), ctB64, nonceB64, req.Enabled)
	if err != nil {
		http.Error(w, `{"error":"保存 OAuth 配置失败"}`, http.StatusInternalServerError)
		return
	}

	s := g.lookup(r)
	actor := "admin"
	if s != nil {
		actor = s.User
	}
	g.recordAudit("CONFIG_UPDATED", actor, req.Provider, g.getClientIP(r),
		fmt.Sprintf("更新第三方 OAuth 配置: provider=%s enabled=%v", req.Provider, req.Enabled))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "OAuth 配置已使用 AES-256-GCM 安全密文存储并生效",
	})
}

// ---------- 6. 辅助工具与第三方协议换取实现 ----------

func generateOAuthState() (state, sig string) {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state = hex.EncodeToString(b) + "." + strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, auth.DeriveMasterKey())
	mac.Write([]byte(state))
	sig = hex.EncodeToString(mac.Sum(nil))
	return state, sig
}

func verifyOAuthState(cookieVal, receivedState string) bool {
	parts := strings.Split(cookieVal, ".")
	if len(parts) != 3 {
		return false
	}
	expectedState := parts[0] + "." + parts[1]
	sig := parts[2]
	if expectedState != receivedState {
		return false
	}

	// 检查 5 分钟时效
	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix()-ts > 300 {
		return false
	}

	mac := hmac.New(sha256.New, auth.DeriveMasterKey())
	mac.Write([]byte(expectedState))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expectedSig))
}

func createRegistrationTicket(profile *ProviderProfile) (string, error) {
	ticket := RegistrationTicket{
		Provider:          profile.Provider,
		UID:               profile.UID,
		Email:             profile.Email,
		SuggestedUsername: profile.SuggestedUsername,
		ExpiresAt:         time.Now().Add(5 * time.Minute).Unix(),
	}
	data, err := json.Marshal(ticket)
	if err != nil {
		return "", err
	}

	masterKey := auth.DeriveMasterKey()
	ct, nonce, err := auth.EncryptSecret(string(data), masterKey)
	if err != nil {
		return "", err
	}
	return ct + "." + nonce, nil
}

func parseRegistrationTicket(ticketStr string) (*RegistrationTicket, error) {
	parts := strings.Split(ticketStr, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid ticket format")
	}

	masterKey := auth.DeriveMasterKey()
	dataStr, err := auth.DecryptSecret(parts[0], parts[1], masterKey)
	if err != nil {
		return nil, errors.New("decrypt ticket failed")
	}

	var ticket RegistrationTicket
	if err := json.Unmarshal([]byte(dataStr), &ticket); err != nil {
		return nil, errors.New("unmarshal ticket json failed")
	}

	if time.Now().Unix() > ticket.ExpiresAt {
		return nil, errors.New("registration ticket has expired")
	}

	return &ticket, nil
}

func sanitizeUsername(input string) string {
	var sb strings.Builder
	for _, ch := range strings.ToLower(input) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			sb.WriteRune(ch)
		}
	}
	res := sb.String()
	if len(res) < 3 {
		res = "user_" + res
		if len(res) < 3 {
			res = "user_box"
		}
	}
	if len(res) > 30 {
		res = res[:30]
	}
	return res
}

func exchangeGitHubProfile(code, clientID, clientSecret, redirectURI string) (*ProviderProfile, error) {
	// 1. 换取 Token
	tokenURL := "https://github.com/login/oauth/access_token"
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github token: %w", err)
	}
	defer resp.Body.Close()

	var tokenRes struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenRes); err != nil {
		return nil, fmt.Errorf("decode github token response: %w", err)
	}
	if tokenRes.Error != "" || tokenRes.AccessToken == "" {
		return nil, fmt.Errorf("github token error: %s", tokenRes.Error)
	}

	// 2. 拉取 User Profile
	userReq, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	userReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
	userReq.Header.Set("User-Agent", "AttackBox-Gateway")

	userResp, err := client.Do(userReq)
	if err != nil {
		return nil, fmt.Errorf("request github user: %w", err)
	}
	defer userResp.Body.Close()

	var ghUser struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("decode github user response: %w", err)
	}

	email := ghUser.Email
	if email == "" {
		// 若主邮箱未公开，调 /user/emails
		emailsReq, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		emailsReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
		emailsReq.Header.Set("User-Agent", "AttackBox-Gateway")
		if eResp, err := client.Do(emailsReq); err == nil {
			defer eResp.Body.Close()
			var emails []struct {
				Email    string `json:"email"`
				Primary  bool   `json:"primary"`
				Verified bool   `json:"verified"`
			}
			if err := json.NewDecoder(eResp.Body).Decode(&emails); err == nil {
				for _, em := range emails {
					if em.Primary && em.Verified {
						email = em.Email
						break
					}
				}
			}
		}
	}

	return &ProviderProfile{
		Provider:          "github",
		UID:               strconv.FormatInt(ghUser.ID, 10),
		Email:             email,
		SuggestedUsername: sanitizeUsername(ghUser.Login),
	}, nil
}

func exchangeMicrosoftProfile(code, clientID, clientSecret, redirectURI string) (*ProviderProfile, error) {
	// 1. 换取 Token
	tokenURL := "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request microsoft token: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var tokenRes struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(bodyBytes, &tokenRes); err != nil {
		return nil, fmt.Errorf("decode microsoft token response: %w", err)
	}
	if tokenRes.Error != "" || tokenRes.AccessToken == "" {
		return nil, fmt.Errorf("microsoft token error: %s (%s)", tokenRes.Error, tokenRes.ErrorDesc)
	}

	// 2. 拉取 Microsoft Graph Profile
	meReq, _ := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)

	meResp, err := client.Do(meReq)
	if err != nil {
		return nil, fmt.Errorf("request microsoft graph: %w", err)
	}
	defer meResp.Body.Close()

	var msUser struct {
		ID                string `json:"id"`
		UserPrincipalName string `json:"userPrincipalName"`
		Mail              string `json:"mail"`
		DisplayName       string `json:"displayName"`
	}
	if err := json.NewDecoder(meResp.Body).Decode(&msUser); err != nil {
		return nil, fmt.Errorf("decode microsoft graph response: %w", err)
	}

	email := msUser.Mail
	if email == "" {
		email = msUser.UserPrincipalName
	}

	prefix := email
	if idx := strings.Index(email, "@"); idx > 0 {
		prefix = email[:idx]
	}

	return &ProviderProfile{
		Provider:          "outlook",
		UID:               msUser.ID,
		Email:             email,
		SuggestedUsername: sanitizeUsername(prefix),
	}, nil
}
