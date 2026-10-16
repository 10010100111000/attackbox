package gateway

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// ---------- 认证 API ----------

type authReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (g *Gateway) apiLogin(w http.ResponseWriter, r *http.Request) {
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}

	u, err := g.authStore.Authenticate(req.Username, req.Password)
	if err != nil {
		logAuth("login failed: user=%s ip=%s reason=%v", req.Username, r.RemoteAddr, err)
		writeJSON(w, map[string]any{"success": false, "error": "用户名或密码错误"})
		return
	}
	logAuth("login success: user=%s role=%s ip=%s", u.Username, u.Role, r.RemoteAddr)

	tok := randHex(32)
	g.mu.Lock()
	g.sess[tok] = &session{
		User:    u.Username,
		Role:    u.Role,
		Expires: time.Now().Add(24 * time.Hour),
	}
	g.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "ab",
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	writeJSON(w, map[string]any{
		"success": true,
		"user": map[string]any{
			"username": u.Username,
			"role":     u.Role,
		},
	})
}

func (g *Gateway) apiRegister(w http.ResponseWriter, r *http.Request) {
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}

	if len(req.Username) < 3 || len(req.Password) < 6 {
		writeJSON(w, map[string]any{"success": false, "error": "用户名至少3位，密码至少6位"})
		return
	}

	u, err := g.authStore.Register(req.Username, req.Password)
	if err != nil {
		logAuth("register failed: user=%s ip=%s reason=%v", req.Username, r.RemoteAddr, err)
		writeJSON(w, map[string]any{"success": false, "error": "该用户名已被注册"})
		return
	}
	logAuth("register success: user=%s role=%s ip=%s", u.Username, u.Role, r.RemoteAddr)

	tok := randHex(32)
	g.mu.Lock()
	g.sess[tok] = &session{
		User:    u.Username,
		Role:    u.Role,
		Expires: time.Now().Add(24 * time.Hour),
	}
	g.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "ab",
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	writeJSON(w, map[string]any{
		"success": true,
		"user": map[string]any{
			"username": u.Username,
			"role":     u.Role,
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
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "ab",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
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
	var ttlMinutes int
	var expiresInSeconds int64

	if b != nil {
		envRunning, guiRunning = g.boxes.DetailedStatus(r.Context(), b)
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
		"loggedIn":   true,
		"user":       map[string]any{"username": s.User, "role": s.Role},
		"boxId":      boxID,
		"cliRunning": envRunning, // 保持向后兼容前端 dash.js
		"guiRunning": guiRunning,
		"envRunning": envRunning,
		"ttlMinutes": ttlMinutes,
		"expiresIn":  expiresInSeconds,
		"broadcast":  g.broadcast,
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
		writeJSON(w, map[string]any{"success": false, "error": "启动沙箱环境失败: " + err.Error()})
		return
	}

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
		writeJSON(w, map[string]any{"success": false, "error": "启动 GUI 桌面失败: " + err.Error()})
		return
	}

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

	runningCount := 0
	for i := range g.cfg.Boxes {
		if g.boxes.Status(r.Context(), &g.cfg.Boxes[i]).Running() {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

func (g *Gateway) apiAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}
	if req.Role != "admin" && req.Role != "user" {
		req.Role = "user"
	}
	_, err := g.authStore.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gateway) apiAdminToggleUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}
	newActive, err := g.authStore.ToggleActive(req.Username)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	// 若用户被置为禁用 (active == false)，立即将其连根拔起踢下线并销毁环境！
	if !newActive {
		g.evictUser(r.Context(), req.Username)
	}
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gateway) apiAdminResetPass(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}
	err := g.authStore.UpdatePassword(req.Username, req.Password)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	// 重置密码后强制让旧会话失效，要求使用新密码重新登录
	g.evictUser(r.Context(), req.Username)
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gateway) apiAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}
	err := g.authStore.DeleteUser(req.Username)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	// 用户已被物理删除，立即踢出并释放其占据的靶场资源
	g.evictUser(r.Context(), req.Username)
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gateway) apiAdminBroadcast(w http.ResponseWriter, r *http.Request) {
	var req BroadcastConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效请求"})
		return
	}
	g.mu.Lock()
	g.broadcast = req
	g.mu.Unlock()
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
