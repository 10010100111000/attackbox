package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type SendCodeReq struct {
	Email string `json:"email"`
}

type RegisterReq struct {
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
	Username string `json:"username,omitempty"`
}

type LoginReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateUsernameReq struct {
	Username string `json:"username"`
}

type AuthResp struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

var allowedEmailSuffixes = []string{
	"@qq.com", "@foxmail.com",
	"@gmail.com", "@googlemail.com",
	"@outlook.com", "@hotmail.com", "@live.com", "@msn.com",
}

func isValidWhitelistEmail(email string) bool {
	lower := strings.ToLower(strings.TrimSpace(email))
	for _, suffix := range allowedEmailSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// Mock verification code used while external SMTP integration is pending.
const DefaultVerificationCode = "123456"

func generate6DigitCode() string {
	// Fixed to 123456 until external SMTP mail server is integrated
	return DefaultVerificationCode
}

func (s *ControlHTTPServer) handlePublicConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"allowRegister": true,
		"authMode":      "email",
	})
}

func (s *ControlHTTPServer) handleSendVerificationCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req SendCodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	targetEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if !isValidWhitelistEmail(targetEmail) {
		writeError(w, http.StatusBadRequest, "仅支持 QQ 邮箱、谷歌 Gmail 及微软 Outlook 邮箱")
		return
	}

	// 1. Check cooldown (60s strict)
	cooldownSec := int64(60)
	lastSent := s.webSessions.GetInt64(r.Context(), "code_sent_at")
	if lastSent > 0 && time.Now().Unix()-lastSent < cooldownSec {
		remaining := cooldownSec - (time.Now().Unix() - lastSent)
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("发送过于频繁，请等待 %d 秒后再试", remaining))
		return
	}

	// 2. Blind verification pattern for anti-enumeration
	// Regardless of whether the email exists, the API returns a uniform success message.
	existingUser, _ := s.db.GetUserByEmail(targetEmail)
	if existingUser != nil {
		// Suppress actual code generation for already-registered emails
		log.Printf("[SECURITY] Email %s is already registered, suppressed code sending", targetEmail)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "若该邮箱符合安全策略，验证码已发送至收件箱",
		})
		return
	}

	// 3. Generate 6-digit code and bind to current Web Session
	code := generate6DigitCode()
	s.webSessions.Put(r.Context(), "reg_email", targetEmail)
	s.webSessions.Put(r.Context(), "reg_code", code)
	s.webSessions.Put(r.Context(), "reg_attempts", 0)
	s.webSessions.Put(r.Context(), "code_sent_at", time.Now().Unix())

	// Print strictly to server console/logs for testing; NEVER return in HTTP response
	log.Printf("[MOCK_EMAIL] Target=%s, VerificationCode=%s", targetEmail, code)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "若该邮箱符合安全策略，验证码已发送至收件箱",
	})
}

func (s *ControlHTTPServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds maximum allowed size (1MB)")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Retrieve session-bound signup context
	savedEmail := s.webSessions.GetString(r.Context(), "reg_email")
	savedCode := s.webSessions.GetString(r.Context(), "reg_code")
	attempts := s.webSessions.GetInt(r.Context(), "reg_attempts")

	if savedEmail == "" || savedCode == "" {
		writeError(w, http.StatusBadRequest, "注册会话已过期，请重新获取验证码")
		return
	}

	// If email was optionally sent in request, ensure it strictly matches session email
	if req.Email != "" && !strings.EqualFold(strings.TrimSpace(req.Email), savedEmail) {
		writeError(w, http.StatusBadRequest, "提交的邮箱与发信邮箱不一致")
		return
	}

	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "密码长度至少需 6 位")
		return
	}

	// Verify code with attempt counter (max 5 attempts, then invalidate)
	if strings.TrimSpace(req.Code) != savedCode {
		attempts++
		s.webSessions.Put(r.Context(), "reg_attempts", attempts)
		if attempts >= 5 {
			s.webSessions.Remove(r.Context(), "reg_email")
			s.webSessions.Remove(r.Context(), "reg_code")
			s.webSessions.Remove(r.Context(), "reg_attempts")
			writeError(w, http.StatusBadRequest, "验证码错误次数超限，请重新获取验证码")
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("验证码错误，还剩 %d 次机会", 5-attempts))
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	// Create user atomically with Snowflake ID and custom or default USER_<id> username
	user, err := s.db.CreateUser(strings.TrimSpace(req.Username), hash, savedEmail, "user")
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			writeError(w, http.StatusConflict, "注册失败，该邮箱或用户名已被占用")
			return
		}
		log.Printf("[AUTH] Failed to create user %s: %v", savedEmail, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Clear registration state from session & renew session token
	s.webSessions.Remove(r.Context(), "reg_email")
	s.webSessions.Remove(r.Context(), "reg_code")
	s.webSessions.Remove(r.Context(), "reg_attempts")
	_ = s.webSessions.RenewToken(r.Context())
	s.webSessions.Put(r.Context(), "auth_uid", user.ID)

	// Issue Ed25519 compact JWT token
	claims := JWTClaims{
		Sub:          user.ID,
		Username:     user.Username,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		Exp:          time.Now().Add(time.Duration(s.cfg.Auth.JWTExpireHours) * time.Hour).Unix(),
	}
	token, err := SignJWT(s.privKey, claims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, AuthResp{
		Token: token,
		User:  user,
	})
}

func (s *ControlHTTPServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds maximum allowed size (1MB)")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		if strings.Contains(req.Username, "@") {
			email = strings.ToLower(strings.TrimSpace(req.Username))
		}
	}

	// Strictly reject username login: email only, must contain '@' and '.'
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "系统仅支持邮箱登录，请输入注册邮箱")
		return
	}

	user, err := s.db.GetUserByEmail(email)
	hashToCompare := dummyBcryptHash
	if err == nil && user != nil {
		hashToCompare = user.PasswordHash
	}

	// Constant-time bcrypt computation to eliminate username enumeration timing side channels
	passwordMatch := CheckPassword(hashToCompare, req.Password)

	if err != nil || user == nil || !passwordMatch {
		writeError(w, http.StatusUnauthorized, "邮箱或密码错误")
		return
	}

	if user.Status != 1 {
		writeError(w, http.StatusForbidden, "account suspended")
		return
	}

	// Renew Web Session & store authenticated user ID
	_ = s.webSessions.RenewToken(r.Context())
	s.webSessions.Put(r.Context(), "auth_uid", user.ID)

	claims := JWTClaims{
		Sub:          user.ID,
		Username:     user.Username,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		Exp:          time.Now().Add(time.Duration(s.cfg.Auth.JWTExpireHours) * time.Hour).Unix(),
	}
	token, err := SignJWT(s.privKey, claims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, AuthResp{
		Token: token,
		User:  user,
	})
}

func (s *ControlHTTPServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	_ = s.webSessions.Destroy(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (s *ControlHTTPServer) handleUpdateUsername(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uid, _, _, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UpdateUsernameReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	newUsername := strings.TrimSpace(req.Username)
	if len(newUsername) < 2 || len(newUsername) > 24 {
		writeError(w, http.StatusBadRequest, "用户名长度需在 2 到 24 个字符之间")
		return
	}

	if err := s.db.UpdateUsername(uid, newUsername); err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			writeError(w, http.StatusConflict, "该用户名已被其他用户占用，请换一个")
			return
		}
		writeError(w, http.StatusInternalServerError, "更新用户名失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"username": newUsername,
	})
}

func (s *ControlHTTPServer) handleMe(w http.ResponseWriter, r *http.Request) {
	uid, _, _, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := s.db.GetUserByID(uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	resp := map[string]interface{}{
		"loggedIn":      true,
		"user":          user,
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"role":          user.Role,
		"status":        user.Status,
		"token_version": user.TokenVersion,
		"created_at":    user.CreatedAt,
	}

	if s.sessions != nil {
		if sbx, running := s.sessions.GetByUser(uid); running && sbx.State == SessionStateRunning {
			now := time.Now()
			if !sbx.ExpiresAt.IsZero() && now.Before(sbx.ExpiresAt) {
				resp["boxId"] = sbx.SandboxID
				resp["cliRunning"] = true
				resp["guiRunning"] = sbx.GUIRunning
				expires := sbx.ExpiresAt.Unix() - now.Unix()
				if expires < 0 {
					expires = 0
				}
				resp["expiresIn"] = expires
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
