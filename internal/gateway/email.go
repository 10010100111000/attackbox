package gateway

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"attackbox/internal/auth"
)

// ---------- 1. 严格邮箱白名单校验 ----------

var allowedEmailDomains = []string{
	"qq.com",
	"foxmail.com",
	"gmail.com",
	"googlemail.com",
	"outlook.com",
	"hotmail.com",
	"live.com",
	"msn.com",
}

func isValidWhitelistEmail(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	atIdx := strings.LastIndex(email, "@")
	if atIdx <= 0 || atIdx == len(email)-1 {
		return false
	}
	domain := email[atIdx+1:]
	for _, allowed := range allowedEmailDomains {
		if domain == allowed {
			return true
		}
	}
	return false
}

// ---------- 2. 内存安全验证码与防刷管理器 ----------

type emailVerificationEntry struct {
	Code      string
	ExpiresAt time.Time
}

type emailSecurityManager struct {
	mu          sync.Mutex
	codes       map[string]*emailVerificationEntry // email -> code entry
	emailCool   map[string]time.Time               // email -> last sent time
	ipCool      map[string]time.Time               // ip -> last sent time
}

var emailSecMgr = &emailSecurityManager{
	codes:     make(map[string]*emailVerificationEntry),
	emailCool: make(map[string]time.Time),
	ipCool:    make(map[string]time.Time),
}

func (m *emailSecurityManager) canSend(email, ip string) (bool, int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if last, ok := m.emailCool[email]; ok {
		elapsed := now.Sub(last)
		if elapsed < 60*time.Second {
			return false, int((60*time.Second - elapsed).Seconds())
		}
	}

	if last, ok := m.ipCool[ip]; ok {
		elapsed := now.Sub(last)
		if elapsed < 30*time.Second {
			return false, int((30*time.Second - elapsed).Seconds())
		}
	}

	return true, 0
}

func (m *emailSecurityManager) setVerificationCode(email, ip string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	code := generateNumericCode(6)
	now := time.Now()
	m.codes[email] = &emailVerificationEntry{
		Code:      code,
		ExpiresAt: now.Add(10 * time.Minute), // 10 分钟有效
	}
	m.emailCool[email] = now
	m.ipCool[ip] = now
	return code
}

func (m *emailSecurityManager) verifyCode(email, code string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.codes[email]
	if !ok {
		return false
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(m.codes, email)
		return false
	}
	if entry.Code != strings.TrimSpace(code) {
		return false
	}
	// 验证成功后单次作废
	delete(m.codes, email)
	return true
}

func generateNumericCode(length int) string {
	const digits = "0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		b[i] = digits[n.Int64()]
	}
	return string(b)
}

// ---------- 3. 安全内存保管库 (Memory Vault Ring) ----------

type memoryVault struct {
	mu        sync.RWMutex
	masterKey []byte
	unsealed  bool
}

var globalVault = &memoryVault{}

func (v *memoryVault) unseal(adminPassword string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.masterKey = auth.DeriveKeyFromPassword(adminPassword)
	v.unsealed = true
}

func (v *memoryVault) getKey() []byte {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.unsealed && len(v.masterKey) == 32 {
		return v.masterKey
	}
	// 退化兼容主机静态派生
	return auth.DeriveMasterKey()
}

// ---------- 4. Resend.com 邮件分发客户端 ----------

type resendSendReq struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

func sendResendEmail(apiKey, fromEmail, toEmail, subject, htmlBody string) error {
	payload := resendSendReq{
		From:    fromEmail,
		To:      []string{toEmail},
		Subject: subject,
		HTML:    htmlBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AttackBox-Gateway/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("resend request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errData struct {
			Message string `json:"message"`
			Name    string `json:"name"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errData)
		return fmt.Errorf("resend api error (%d): %s", resp.StatusCode, errData.Message)
	}

	return nil
}

// ---------- 5. API 端点实现 ----------

type sendVerificationCodeReq struct {
	Email string `json:"email"`
}

func (g *Gateway) apiSendVerificationCode(w http.ResponseWriter, r *http.Request) {
	// 系统是否开放自主注册
	if settings, err := g.authStore.GetAllSettings(); err == nil {
		if allow, ok := settings["allow_register"]; ok && allow == "false" {
			writeJSON(w, map[string]any{"success": false, "error": "管理员已关闭自主注册功能"})
			return
		}
	}

	clientIP := g.getClientIP(r)

	var req sendVerificationCodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}

	targetEmail := strings.ToLower(strings.TrimSpace(req.Email))

	// 1. 严格白名单域名校验
	if !isValidWhitelistEmail(targetEmail) {
		writeJSON(w, map[string]any{
			"success": false,
			"error":   "邮箱格式不支持。系统目前仅支持 QQ 邮箱 (@qq.com, @foxmail.com)、谷歌邮箱 (@gmail.com) 以及 Outlook 邮箱 (@outlook.com, @hotmail.com 等)。",
		})
		return
	}

	// 2. 检查邮箱是否已被注册占用
	if existing, _ := g.authStore.GetUserByEmail(targetEmail); existing != nil {
		writeJSON(w, map[string]any{"success": false, "error": "该邮箱已被其他账号绑定，请直接登录或更换邮箱"})
		return
	}

	// 3. 防刷频控与冷却校验
	allowed, waitSec := emailSecMgr.canSend(targetEmail, clientIP)
	if !allowed {
		writeJSON(w, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("发送过于频繁，请等待 %d 秒后再试", waitSec),
		})
		return
	}

	// 4. 读取密文并瞬时解密 Resend API Key
	settings, err := g.authStore.GetAllSettings()
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "读取系统设置异常"})
		return
	}

	apiKeyEnc := settings["resend_api_key_enc"]
	nonceEnc := settings["resend_api_key_nonce"]
	fromEmail := settings["resend_from_email"]
	enabled := settings["resend_enabled"] == "true"

	if !enabled || apiKeyEnc == "" || nonceEnc == "" {
		writeJSON(w, map[string]any{
			"success": false,
			"error":   "系统尚未配置邮件发送服务，请联系平台管理员",
		})
		return
	}

	if fromEmail == "" {
		fromEmail = "AttackBox <onboarding@resend.dev>"
	}

	vaultKey := globalVault.getKey()
	resendKey, err := auth.DecryptSecret(apiKeyEnc, nonceEnc, vaultKey)
	if err != nil {
		logAuth("decrypt resend key failed: %v", err)
		writeJSON(w, map[string]any{
			"success": false,
			"error":   "解密邮件服务凭据失败，需管理员重新登入系统解封",
		})
		return
	}

	// 5. 生成 6 位随机验证码并调用 Resend API 发信
	code := emailSecMgr.setVerificationCode(targetEmail, clientIP)

	subject := fmt.Sprintf("【AttackBox】注册验证码: %s", code)
	htmlBody := fmt.Sprintf(`
		<div style="font-family: monospace, sans-serif; max-width: 480px; margin: 0 auto; padding: 24px; border: 1px solid #e4e4e7; border-radius: 8px;">
			<h2 style="font-size: 16px; color: #18181b; margin-bottom: 12px;">AttackBox 靶场工作台</h2>
			<p style="font-size: 13px; color: #52525b; line-height: 1.6;">您正在申请注册 AttackBox 账户，本次注册验证码为：</p>
			<div style="background: #f4f4f5; padding: 14px; text-align: center; font-size: 24px; font-weight: bold; letter-spacing: 4px; color: #09090b; border-radius: 6px; margin: 16px 0;">
				%s
			</div>
			<p style="font-size: 12px; color: #71717a;">验证码有效期为 10 分钟。若非您本人操作，请忽略此邮件。</p>
		</div>
	`, code)

	if err := sendResendEmail(resendKey, fromEmail, targetEmail, subject, htmlBody); err != nil {
		logAuth("resend send email to %s failed: %v", targetEmail, err)
		writeJSON(w, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("邮件投递失败: %v", err),
		})
		return
	}

	g.recordAudit("EMAIL_CODE_SENT", targetEmail, "resend", clientIP, "验证码邮件已投递")

	writeJSON(w, map[string]any{
		"success": true,
		"message": "验证码已成功发送至您的邮箱，10分钟内有效",
	})
}

// ---------- 6. 管理端 Resend 配置 API ----------

func (g *Gateway) apiAdminResendGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := g.authStore.GetAllSettings()
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "读取设置失败"})
		return
	}

	configured := settings["resend_api_key_enc"] != ""
	fromEmail := settings["resend_from_email"]
	if fromEmail == "" {
		fromEmail = "AttackBox <onboarding@resend.dev>"
	}
	enabled := settings["resend_enabled"] == "true"

	writeJSON(w, map[string]any{
		"success": true,
		"config": map[string]any{
			"configured": configured,
			"enabled":    enabled,
			"from":       fromEmail,
		},
	})
}

type adminResendUpdateReq struct {
	APIKey string `json:"api_key"`
	From   string `json:"from"`
	Enabled bool   `json:"enabled"`
}

func (g *Gateway) apiAdminResendUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req adminResendUpdateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}

	fromEmail := strings.TrimSpace(req.From)
	if fromEmail == "" {
		fromEmail = "AttackBox <onboarding@resend.dev>"
	}

	enabledStr := "false"
	if req.Enabled {
		enabledStr = "true"
	}
	_ = g.authStore.SetSetting("resend_enabled", enabledStr)
	_ = g.authStore.SetSetting("resend_from_email", fromEmail)

	if strings.TrimSpace(req.APIKey) != "" {
		vaultKey := globalVault.getKey()
		ct, nonce, err := auth.EncryptSecret(strings.TrimSpace(req.APIKey), vaultKey)
		if err != nil {
			writeJSON(w, map[string]any{"success": false, "error": "加密 Resend API Key 失败: " + err.Error()})
			return
		}
		_ = g.authStore.SetSetting("resend_api_key_enc", ct)
		_ = g.authStore.SetSetting("resend_api_key_nonce", nonce)
	}

	s := g.lookup(r)
	actor := "admin"
	if s != nil {
		actor = s.User
	}
	g.recordAudit("CONFIG_UPDATED", actor, "resend_settings", g.getClientIP(r), "更新 Resend 邮件分发服务配置")

	writeJSON(w, map[string]any{
		"success": true,
		"message": "Resend 邮件配置已通过 AES-256-GCM 密文保存并生效",
	})
}
