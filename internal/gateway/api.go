package gateway

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"attackbox/internal/auth"
	"attackbox/internal/config"

	"github.com/gorilla/websocket"
)

var validUsername = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

// ---------- 认证 API ----------

type authReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (g *Gateway) apiLogin(w http.ResponseWriter, r *http.Request) {
	clientIP := g.getClientIP(r)
	if !g.limiter.allow(clientIP) {
		http.Error(w, `{"success":false,"error":"尝试登录次数过多，请稍后再试"}`, http.StatusTooManyRequests)
		return
	}

	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}

	if !validUsername.MatchString(req.Username) {
		g.limiter.recordFailure(clientIP)
		g.recordAudit("LOGIN_FAILED", req.Username, "auth", clientIP, "用户名格式不合法")
		writeJSON(w, map[string]any{"success": false, "error": "用户名或密码错误"})
		return
	}

	u, err := g.authStore.Authenticate(req.Username, req.Password)
	if err != nil {
		g.limiter.recordFailure(clientIP)
		logAuth("login failed: user=%s ip=%s reason=%v", req.Username, clientIP, err)
		g.recordAudit("LOGIN_FAILED", req.Username, "auth", clientIP, fmt.Sprintf("登录失败: %v", err))
		writeJSON(w, map[string]any{"success": false, "error": "用户名或密码错误"})
		return
	}
	g.limiter.reset(clientIP)
	logAuth("login success: user=%s role=%s ip=%s", u.Username, u.Role, clientIP)

	// 若管理员成功登录，自动使用密码解封安全内存保管库 (Unseal Vault)
	if u.Role == "admin" {
		globalVault.unseal(req.Password)
	}

	tok := randHex(32)
	expires := time.Now().Add(24 * time.Hour)
	g.mu.Lock()
	g.sess[tok] = &session{
		User:    u.Username,
		Role:    u.Role,
		Expires: expires,
	}
	g.mu.Unlock()
	_ = g.authStore.SaveSession(tok, u.Username, u.Role, expires)

	http.SetCookie(w, &http.Cookie{
		Name:     "ab",
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})

	writeJSON(w, map[string]any{
		"success": true,
		"user": map[string]any{
			"username": u.Username,
			"role":     u.Role,
		},
	})
}

type userRegisterReq struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Email         string `json:"email"`
	Code          string `json:"code"`
	AffCode       string `json:"affCode"`
	AffCodeSnake  string `json:"aff_code"`
}

func (g *Gateway) apiRegister(w http.ResponseWriter, r *http.Request) {
	clientIP := g.getClientIP(r)
	allowRegStr := g.authStore.GetSetting("allow_register", "true")
	if allowRegStr == "false" {
		writeJSON(w, map[string]any{"success": false, "error": "管理员已关闭自主注册功能"})
		return
	}

	var req userRegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}

	if !validUsername.MatchString(req.Username) || len(req.Password) < 6 {
		writeJSON(w, map[string]any{"success": false, "error": "用户名需3-32位字母数字下划线或连字符，密码至少6位"})
		return
	}

	targetEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if !isValidWhitelistEmail(targetEmail) {
		writeJSON(w, map[string]any{
			"success": false,
			"error":   "邮箱不支持，仅支持 QQ 邮箱 (@qq.com, @foxmail.com)、谷歌邮箱 (@gmail.com) 或 Outlook 邮箱",
		})
		return
	}

	// 检查邮箱是否已被占用
	if existing, _ := g.authStore.GetUserByEmail(targetEmail); existing != nil {
		writeJSON(w, map[string]any{"success": false, "error": "该邮箱已被其他账号绑定，请直接登录"})
		return
	}

	// 检查 AFF 邀请码 (可选，兼容 affCode 与 aff_code)
	invitedByCode := strings.TrimSpace(req.AffCode)
	if invitedByCode == "" {
		invitedByCode = strings.TrimSpace(req.AffCodeSnake)
	}
	if invitedByCode != "" {
		inviter, _ := g.authStore.GetUserByAffCode(invitedByCode)
		if inviter == nil {
			writeJSON(w, map[string]any{"success": false, "error": "所填写的邀请码无效或不存在"})
			return
		}
	}

	// 核验邮箱验证码
	if !emailSecMgr.verifyCode(targetEmail, req.Code) {
		writeJSON(w, map[string]any{"success": false, "error": "邮箱验证码无效或已过期，请重新获取"})
		return
	}

	u, err := g.authStore.CreateUserWithEmailAndAFF(req.Username, req.Password, "user", targetEmail, invitedByCode)
	if err != nil {
		logAuth("register failed: user=%s ip=%s reason=%v", req.Username, clientIP, err)
		if errors.Is(err, auth.ErrUserExists) {
			writeJSON(w, map[string]any{"success": false, "error": "该用户名已被注册"})
		} else {
			writeJSON(w, map[string]any{"success": false, "error": "注册失败: " + err.Error()})
		}
		return
	}
	logAuth("register success: user=%s role=%s email=%s aff=%s ip=%s", u.Username, u.Role, u.Email, u.AffCode, clientIP)
	g.recordAudit("USER_REGISTER", u.Username, u.Username, clientIP, fmt.Sprintf("自主注册成功: email=%s invited_by=%s", u.Email, u.InvitedBy))

	tok := randHex(32)
	expires := time.Now().Add(24 * time.Hour)
	g.mu.Lock()
	g.sess[tok] = &session{
		User:    u.Username,
		Role:    u.Role,
		Expires: expires,
	}
	g.mu.Unlock()
	_ = g.authStore.SaveSession(tok, u.Username, u.Role, expires)

	http.SetCookie(w, &http.Cookie{
		Name:     "ab",
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})

	writeJSON(w, map[string]any{
		"success": true,
		"user": map[string]any{
			"username": u.Username,
			"role":     u.Role,
			"email":    u.Email,
			"affCode":  u.AffCode,
		},
	})
}

func (g *Gateway) apiLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("ab")
	if err == nil {
		g.mu.Lock()
		if s, ok := g.sess[c.Value]; ok {
			logAuth("logout: user=%s", s.User)
		}
		delete(g.sess, c.Value)
		g.mu.Unlock()
		_ = g.authStore.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "ab",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
	})
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gateway) logout(w http.ResponseWriter, r *http.Request) {
	g.apiLogout(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (g *Gateway) apiMe(w http.ResponseWriter, r *http.Request) {
	s := g.lookup(r)
	if s == nil {
		writeJSON(w, map[string]any{"loggedIn": false})
		return
	}

	b, _ := g.currentBoxFor(r)
	envRunning := false
	guiRunning := false
	boxID := ""
	var expiresInSeconds int64

	if b != nil {
		envRunning, guiRunning = g.boxes.DetailedStatus(r.Context(), b)
		boxID = b.ID
	}

	g.mu.Lock()
	ttlMinutes := g.cfg.TTLMinutes
	if ttlMinutes <= 0 && b != nil {
		ttlMinutes = b.TTLMinutes
	}
	idleTimeout := g.cfg.IdleTimeoutMinutes
	broadcast := g.broadcast

	if lease, ok := g.userLeases[s.User]; ok && lease != nil {
		rem := time.Until(lease.ExpiresAt).Seconds()
		if rem > 0 {
			expiresInSeconds = int64(rem)
		}
	}
	g.mu.Unlock()

	userMap := map[string]any{"username": s.User, "role": s.Role}
	if u, err := g.authStore.GetUser(s.User); err == nil && u != nil {
		userMap["email"] = u.Email
		userMap["affCode"] = u.AffCode
	}

	writeJSON(w, map[string]any{
		"loggedIn":           true,
		"user":               userMap,
		"boxId":              boxID,
		"cliRunning":         envRunning, // 保持向后兼容前端 dash.js
		"guiRunning":         guiRunning,
		"envRunning":         envRunning,
		"ttlMinutes":         ttlMinutes,
		"idleTimeoutMinutes": idleTimeout,
		"expiresIn":          expiresInSeconds,
		"broadcast":          broadcast,
	})
}

// ---------- 渗透环境控制 API ----------

// apiCliToggle 确保沙箱基础环境就绪（向后兼容前端 openCliWindow）
func (g *Gateway) apiCliToggle(w http.ResponseWriter, r *http.Request) {
	s := g.lookup(r)
	if s == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	allocatedBox, _, err := g.acquireBoxFor(r)
	if err != nil || allocatedBox == nil {
		writeJSON(w, map[string]any{"success": false, "error": "靶场沙箱槽位已满，暂无空闲资源，请稍后再试或联系管理员"})
		return
	}

	if err := g.boxes.Ensure(ctx, allocatedBox); err != nil {
		logEnv("start sandbox failed: user=%s box=%s err=%v", s.User, allocatedBox.ID, err)
		writeJSON(w, map[string]any{"success": false, "error": "启动沙箱环境失败，请重试或联系管理员"})
		return
	}

	g.touchBoxActivity(allocatedBox.ID)
	_, guiRunning := g.boxes.DetailedStatus(ctx, allocatedBox)
	writeJSON(w, map[string]any{"success": true, "envRunning": true, "cliRunning": true, "guiRunning": guiRunning})
}

// apiGuiToggle 独立控制沙箱内部的 GUI 桌面服务 (TigerVNC + MATE)，与底座沙箱及命令行会话完全解耦
func (g *Gateway) apiGuiToggle(w http.ResponseWriter, r *http.Request) {
	s := g.lookup(r)
	if s == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	allocatedBox, _, err := g.acquireBoxFor(r)
	if err != nil || allocatedBox == nil {
		writeJSON(w, map[string]any{"success": false, "error": "靶场沙箱槽位已满，暂无空闲资源"})
		return
	}

	envRunning, guiRunning := g.boxes.DetailedStatus(ctx, allocatedBox)
	if guiRunning {
		// 仅停用桌面服务，沙箱底座及进行中的 CLI 任务完全不受影响
		logGUI("stop desktop requested: user=%s box=%s", s.User, allocatedBox.ID)
		g.guiMu.Lock()
		if conn, ok := g.activeGUI[allocatedBox.ID]; ok && conn != nil {
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "gui stopped"),
				time.Now().Add(time.Second),
			)
			_ = conn.Close()
			delete(g.activeGUI, allocatedBox.ID)
		}
		g.guiMu.Unlock()

		_ = g.boxes.StopGUI(ctx, allocatedBox)
		writeJSON(w, map[string]any{"success": true, "envRunning": envRunning, "cliRunning": envRunning, "guiRunning": false})
		return
	}

	// 按需拉起图形桌面服务
	logGUI("start desktop requested: user=%s box=%s", s.User, allocatedBox.ID)
	if err := g.boxes.EnsureGUI(ctx, allocatedBox); err != nil {
		logGUI("ensure desktop failed: user=%s box=%s err=%v", s.User, allocatedBox.ID, err)
		writeJSON(w, map[string]any{"success": false, "error": "启动 GUI 桌面失败，请重试或联系管理员"})
		return
	}

	g.touchBoxActivity(allocatedBox.ID)
	writeJSON(w, map[string]any{"success": true, "envRunning": true, "cliRunning": true, "guiRunning": true})
}

func (g *Gateway) closeBoxSessions(boxID string) {
	g.cliMu.Lock()
	for key, conn := range g.activeCLI {
		if strings.HasPrefix(key, boxID+":") || key == boxID {
			if conn != nil {
				_ = conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "environment destroyed"),
					time.Now().Add(time.Second),
				)
				_ = conn.Close()
			}
			delete(g.activeCLI, key)
		}
	}
	g.cliMu.Unlock()

	g.guiMu.Lock()
	if conn, ok := g.activeGUI[boxID]; ok && conn != nil {
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "environment destroyed"),
			time.Now().Add(time.Second),
		)
		_ = conn.Close()
		delete(g.activeGUI, boxID)
	}
	g.guiMu.Unlock()
}

func (g *Gateway) apiEnvDestroy(w http.ResponseWriter, r *http.Request) {
	b, s := g.currentBoxFor(r)
	if s == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if b == nil {
		writeJSON(w, map[string]any{"success": true, "cliRunning": false, "guiRunning": false, "envRunning": false})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	logEnv("destroy environment requested: user=%s box=%s", s.User, b.ID)
	g.closeBoxSessions(b.ID)
	_ = g.boxes.Kick(ctx, b)

	g.mu.Lock()
	delete(g.userLeases, s.User)
	delete(g.boxActivity, b.ID)
	g.mu.Unlock()

	writeJSON(w, map[string]any{"success": true, "cliRunning": false, "guiRunning": false, "envRunning": false})
}

func (g *Gateway) apiEnvStatus(w http.ResponseWriter, r *http.Request) {
	b, s := g.currentBoxFor(r)
	if s == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	running := false
	guiRunning := false
	boxID := ""
	var expiresInSeconds int64
	var ttlMinutes int

	if b != nil {
		running, guiRunning = g.boxes.DetailedStatus(r.Context(), b)
		boxID = b.ID
		ttlMinutes = b.TTLMinutes

		g.mu.Lock()
		if lease, ok := g.userLeases[s.User]; ok && lease != nil {
			rem := time.Until(lease.ExpiresAt).Seconds()
			if rem > 0 {
				expiresInSeconds = int64(rem)
			}
		}
		g.mu.Unlock()
	}

	writeJSON(w, map[string]any{
		"running":    running,
		"envRunning": running,
		"cliRunning": running,
		"guiRunning": guiRunning,
		"box":        boxID,
		"boxId":      boxID,
		"ttlMinutes": ttlMinutes,
		"expiresIn":  expiresInSeconds,
	})
}

// ---------- 管理员 API ----------

func (g *Gateway) apiAdminMetrics(w http.ResponseWriter, r *http.Request) {
	cpuPercent := getCPUPercent()
	memUsed, memTotal := getMemUsageGB()
	diskUsed, diskTotal := getDiskUsageGB("/var/lib/attackbox")

	g.mu.Lock()
	boxesSnapshot := make([]config.Box, len(g.cfg.Boxes))
	copy(boxesSnapshot, g.cfg.Boxes)
	g.mu.Unlock()

	runningCount := 0
	for i := range boxesSnapshot {
		if g.boxes.Status(r.Context(), &boxesSnapshot[i]).Running() {
			runningCount++
		}
	}

	writeJSON(w, map[string]any{
		"vpsOnline":    true,
		"cpuPercent":   cpuPercent,
		"memUsedGB":    memUsed,
		"memTotalGB":   memTotal,
		"diskUsedGB":   diskUsed,
		"diskTotalGB":  diskTotal,
		"runningBoxes": runningCount,
	})
}

func (g *Gateway) apiAdminListUsers(w http.ResponseWriter, r *http.Request) {
	list, err := g.authStore.ListUsers()
	if err != nil {
		logSec("admin list users error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

func (g *Gateway) apiAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	s := g.lookup(r)
	adminUser := "admin"
	if s != nil {
		adminUser = s.User
	}
	clientIP := g.getClientIP(r)

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}
	if !validUsername.MatchString(req.Username) || len(req.Password) < 6 {
		writeJSON(w, map[string]any{"success": false, "error": "用户名需3-32位字母数字下划线或连字符，密码至少6位"})
		return
	}
	if req.Role != "admin" && req.Role != "user" {
		req.Role = "user"
	}
	_, err := g.authStore.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			writeJSON(w, map[string]any{"success": false, "error": "该用户名已存在"})
		} else {
			logSec("admin create user failed: %v", err)
			writeJSON(w, map[string]any{"success": false, "error": "创建用户失败"})
		}
		return
	}
	g.recordAudit("USER_CREATED", adminUser, req.Username, clientIP, fmt.Sprintf("管理员创建账号 (角色: %s)", req.Role))
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gateway) apiAdminToggleUser(w http.ResponseWriter, r *http.Request) {
	s := g.lookup(r)
	adminUser := "admin"
	if s != nil {
		adminUser = s.User
	}
	clientIP := g.getClientIP(r)

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}

	targetUser, err := g.authStore.GetUser(req.Username)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "用户不存在"})
		return
	}

	// 杜绝自锁与失去最后一个活跃管理员
	if s != nil && s.User == req.Username && targetUser.Active {
		writeJSON(w, map[string]any{"success": false, "error": "禁止操作：无法禁用当前正在登录的管理员账号"})
		return
	}

	if targetUser.Role == "admin" && targetUser.Active {
		adminCount, err := g.authStore.CountActiveAdmins()
		if err != nil || adminCount <= 1 {
			writeJSON(w, map[string]any{"success": false, "error": "禁止操作：系统必须保留至少一个处于启用状态的管理员账号"})
			return
		}
	}

	newActive, err := g.authStore.ToggleActive(req.Username)
	if err != nil {
		logSec("admin toggle user %s error: %v", req.Username, err)
		writeJSON(w, map[string]any{"success": false, "error": "更新用户状态失败"})
		return
	}
	// 若用户被置为禁用 (active == false)，立即将其连根拔起踢下线并销毁环境！
	actionStr := "启用用户"
	if !newActive {
		actionStr = "禁用用户 (已级联终止沙箱与会话)"
		g.evictUser(r.Context(), req.Username)
	}
	g.recordAudit("USER_TOGGLE", adminUser, req.Username, clientIP, actionStr)
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gateway) apiAdminResetPass(w http.ResponseWriter, r *http.Request) {
	s := g.lookup(r)
	adminUser := "admin"
	if s != nil {
		adminUser = s.User
	}
	clientIP := g.getClientIP(r)

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}
	if len(req.Password) < 6 {
		writeJSON(w, map[string]any{"success": false, "error": "密码长度至少6位"})
		return
	}
	err := g.authStore.UpdatePassword(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeJSON(w, map[string]any{"success": false, "error": "用户不存在"})
		} else {
			logSec("admin reset pass %s error: %v", req.Username, err)
			writeJSON(w, map[string]any{"success": false, "error": "重置密码失败"})
		}
		return
	}
	// 重置密码后强制让旧会话失效，要求使用新密码重新登录
	g.evictUser(r.Context(), req.Username)
	g.recordAudit("PASSWORD_RESET", adminUser, req.Username, clientIP, "管理员重置密码")
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gateway) apiAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	s := g.lookup(r)
	adminUser := "admin"
	if s != nil {
		adminUser = s.User
	}
	clientIP := g.getClientIP(r)

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}

	if s != nil && s.User == req.Username {
		writeJSON(w, map[string]any{"success": false, "error": "禁止操作：无法删除当前正在登录的管理员账号"})
		return
	}

	targetUser, err := g.authStore.GetUser(req.Username)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "用户不存在"})
		return
	}

	// 杜绝删除最后一个管理员
	if targetUser.Role == "admin" {
		adminCount, err := g.authStore.CountActiveAdmins()
		if err != nil || adminCount <= 1 {
			writeJSON(w, map[string]any{"success": false, "error": "禁止删除：系统必须保留至少一个处于启用状态的管理员账号"})
			return
		}
	}

	err = g.authStore.DeleteUser(req.Username)
	if err != nil {
		logSec("admin delete user %s error: %v", req.Username, err)
		writeJSON(w, map[string]any{"success": false, "error": "删除用户失败"})
		return
	}
	// 用户已被物理删除，立即踢出并释放其占据的靶场资源
	g.evictUser(r.Context(), req.Username)
	g.recordAudit("USER_DELETED", adminUser, req.Username, clientIP, "管理员删除用户")
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gateway) apiAdminBroadcast(w http.ResponseWriter, r *http.Request) {
	s := g.lookup(r)
	adminUser := "admin"
	if s != nil {
		adminUser = s.User
	}
	clientIP := g.getClientIP(r)

	var req BroadcastConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效请求"})
		return
	}
	g.mu.Lock()
	g.broadcast = req
	g.mu.Unlock()

	_ = g.authStore.SetSetting("broadcast_enabled", strconv.FormatBool(req.Enabled))
	_ = g.authStore.SetSetting("broadcast_message", req.Message)
	g.recordAudit("BROADCAST_CHANGED", adminUser, "system", clientIP, fmt.Sprintf("管理员修改全员广播 (启用: %v)", req.Enabled))

	writeJSON(w, map[string]any{"success": true})
}

// adminSandboxItem 管理员沙箱监控视图
type adminSandboxItem struct {
	BoxID           string    `json:"box_id"`
	User            string    `json:"user"`
	StartedAt       time.Time `json:"started_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	TTLRemainingSec int64     `json:"ttl_remaining_sec"`
	Running         bool      `json:"running"`
	MainPID         int       `json:"main_pid"`
}

type sandboxSnapshot struct {
	box       *config.Box
	boxID     string
	user      string
	startedAt time.Time
	expiresAt time.Time
}

func (g *Gateway) apiAdminListSandboxes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 锁内仅做轻量级数据快照，微秒级释放全局锁，彻底杜绝 Status 阻塞全局鉴权 lookup
	g.mu.Lock()
	snaps := make([]sandboxSnapshot, 0, len(g.userLeases))
	for username, lease := range g.userLeases {
		snaps = append(snaps, sandboxSnapshot{
			box:       g.cfg.BoxByID(lease.BoxID),
			boxID:     lease.BoxID,
			user:      username,
			startedAt: lease.StartedAt,
			expiresAt: lease.ExpiresAt,
		})
	}
	g.mu.Unlock()

	// 锁外执行外部系统调用与状态检测
	list := make([]adminSandboxItem, 0, len(snaps))
	for _, item := range snaps {
		running := false
		mainPID := 0
		if item.box != nil {
			st := g.boxes.Status(ctx, item.box)
			running = st.Running()
			mainPID = st.MainPID
		}
		rem := int64(time.Until(item.expiresAt).Seconds())
		if rem < 0 {
			rem = 0
		}
		list = append(list, adminSandboxItem{
			BoxID:           item.boxID,
			User:            item.user,
			StartedAt:       item.startedAt,
			ExpiresAt:       item.expiresAt,
			TTLRemainingSec: rem,
			Running:         running,
			MainPID:         mainPID,
		})
	}
	writeJSON(w, list)
}

func (g *Gateway) apiAdminReleaseSandbox(w http.ResponseWriter, r *http.Request) {
	var req struct {
		User  string `json:"user"`
		BoxID string `json:"box_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效请求"})
		return
	}

	s := g.lookup(r)
	adminUser := "admin"
	if s != nil {
		adminUser = s.User
	}
	clientIP := g.getClientIP(r)

	g.mu.Lock()
	var targetUser string
	var targetBoxID string
	var boxToKick *config.Box

	if req.User != "" {
		if lease, ok := g.userLeases[req.User]; ok {
			targetUser = req.User
			targetBoxID = lease.BoxID
			boxToKick = g.cfg.BoxByID(lease.BoxID)
			delete(g.userLeases, req.User)
			delete(g.boxActivity, lease.BoxID)
		}
	} else if req.BoxID != "" {
		for u, lease := range g.userLeases {
			if lease.BoxID == req.BoxID {
				targetUser = u
				targetBoxID = req.BoxID
				boxToKick = g.cfg.BoxByID(req.BoxID)
				delete(g.userLeases, u)
				delete(g.boxActivity, req.BoxID)
				break
			}
		}
	}
	g.mu.Unlock()

	if targetUser == "" && targetBoxID == "" {
		writeJSON(w, map[string]any{"success": false, "error": "未找到运行中的沙箱或租约"})
		return
	}

	if targetBoxID != "" {
		g.closeBoxSessions(targetBoxID)
	}
	if boxToKick != nil {
		_ = g.boxes.Kick(r.Context(), boxToKick)
	}

	g.recordAudit("SANDBOX_FORCE_RELEASED", adminUser, targetUser, clientIP, fmt.Sprintf("强制释放沙箱 box=%s 回收资源", targetBoxID))
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gateway) apiAdminListAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 50
	offset := 0
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(q.Get("offset")); err == nil && o >= 0 {
		offset = o
	}
	eventType := q.Get("event_type")
	search := q.Get("search")

	logs, total, err := g.authStore.ListAuditLogs(limit, offset, eventType, search)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	if logs == nil {
		logs = []auth.AuditLog{}
	}

	writeJSON(w, map[string]any{
		"success": true,
		"total":   total,
		"items":   logs,
	})
}

func (g *Gateway) apiAdminGetSettings(w http.ResponseWriter, r *http.Request) {
	allowRegStr := g.authStore.GetSetting("allow_register", "true")
	allowReg := (allowRegStr != "false")

	g.mu.Lock()
	maxSlots := g.cfg.MaxSlots
	ttlMinutes := g.cfg.TTLMinutes
	idleTimeout := g.cfg.IdleTimeoutMinutes
	g.mu.Unlock()

	writeJSON(w, map[string]any{
		"maxSlots":           maxSlots,
		"ttlMinutes":         ttlMinutes,
		"idleTimeoutMinutes": idleTimeout,
		"allowRegister":      allowReg,
	})
}

func (g *Gateway) apiAdminUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MaxSlots           *int  `json:"maxSlots"`
		TTLMinutes         *int  `json:"ttlMinutes"`
		IdleTimeoutMinutes *int  `json:"idleTimeoutMinutes"`
		AllowRegister      *bool `json:"allowRegister"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效请求"})
		return
	}

	s := g.lookup(r)
	adminUser := "admin"
	if s != nil {
		adminUser = s.User
	}
	clientIP := g.getClientIP(r)

	g.mu.Lock()
	if req.MaxSlots != nil && *req.MaxSlots > 0 && *req.MaxSlots <= 100 {
		g.cfg.MaxSlots = *req.MaxSlots
		g.cfg.EnsureSlots(*req.MaxSlots)
		_ = g.authStore.SetSetting("max_slots", strconv.Itoa(*req.MaxSlots))
	}
	if req.TTLMinutes != nil && *req.TTLMinutes > 0 && *req.TTLMinutes <= 1440 {
		g.cfg.TTLMinutes = *req.TTLMinutes
		_ = g.authStore.SetSetting("ttl_minutes", strconv.Itoa(*req.TTLMinutes))
	}
	if req.IdleTimeoutMinutes != nil && *req.IdleTimeoutMinutes > 0 && *req.IdleTimeoutMinutes <= 120 {
		g.cfg.IdleTimeoutMinutes = *req.IdleTimeoutMinutes
		_ = g.authStore.SetSetting("idle_timeout_minutes", strconv.Itoa(*req.IdleTimeoutMinutes))
	}
	if req.AllowRegister != nil {
		val := "true"
		if !*req.AllowRegister {
			val = "false"
		}
		_ = g.authStore.SetSetting("allow_register", val)
	}
	g.mu.Unlock()

	g.recordAudit("CONFIG_CHANGED", adminUser, "system", clientIP, "更新系统全局配置 (TTL/Slots/Register)")
	writeJSON(w, map[string]any{"success": true})
}

// ---------- 系统监控与工具函数 ----------

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(data)
}

func randHex(n int) string {
	b := make([]byte, n/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func getCPUPercent() int {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 15
	}
	parts := strings.Fields(string(data))
	if len(parts) > 0 {
		load, _ := strconv.ParseFloat(parts[0], 64)
		pct := int(load * 25)
		if pct > 100 {
			pct = 100
		}
		if pct < 5 {
			pct = 5
		}
		return pct
	}
	return 15
}

func getMemUsageGB() (float64, float64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 3.2, 8.0
	}
	defer file.Close()

	var totalKB, availKB float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				totalKB, _ = strconv.ParseFloat(fields[1], 64)
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				availKB, _ = strconv.ParseFloat(fields[1], 64)
			}
		}
	}
	if totalKB == 0 {
		return 3.2, 8.0
	}
	usedKB := totalKB - availKB
	totalGB := math.Round(totalKB/1024/1024*10) / 10
	usedGB := math.Round(usedKB/1024/1024*10) / 10
	return usedGB, totalGB
}

func getDiskUsageGB(path string) (float64, float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 45.0, 100.0
	}
	total := float64(stat.Blocks) * float64(stat.Bsize)
	free := float64(stat.Bfree) * float64(stat.Bsize)
	used := total - free
	totalGB := math.Round(total/(1024*1024*1024)*10) / 10
	usedGB := math.Round(used/(1024*1024*1024)*10) / 10
	return usedGB, totalGB
}

func (g *Gateway) apiUserChangePassword(w http.ResponseWriter, r *http.Request) {
	s := g.lookup(r)
	if s == nil {
		writeJSON(w, map[string]any{"success": false, "error": "未授权访问"})
		return
	}
	clientIP := g.getClientIP(r)

	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}

	if len(req.NewPassword) < 6 {
		writeJSON(w, map[string]any{"success": false, "error": "新密码长度至少6位"})
		return
	}

	// 校验旧密码
	_, err := g.authStore.Authenticate(s.User, req.OldPassword)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "当前旧密码不正确"})
		return
	}

	// 更新为新密码
	if err := g.authStore.UpdatePassword(s.User, req.NewPassword); err != nil {
		logSec("user change password error: %v", err)
		writeJSON(w, map[string]any{"success": false, "error": "更新密码失败"})
		return
	}

	// 改密后级联注销全部会话并回收环境，要求重新登录
	g.evictUser(r.Context(), s.User)
	g.recordAudit("PASSWORD_RESET", s.User, s.User, clientIP, "用户自主修改密码 (已级联注销全部会话)")

	// 清除 Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "ab",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
	})

	writeJSON(w, map[string]any{"success": true})
}

func (g *Gateway) apiPublicConfig(w http.ResponseWriter, r *http.Request) {
	allowRegStr := g.authStore.GetSetting("allow_register", "true")
	writeJSON(w, map[string]any{
		"allowRegister": allowRegStr != "false",
	})
}
