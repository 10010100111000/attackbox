package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuthStoreCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ab-auth-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	// 1. 测试注册普通用户
	u, err := s.Register("alice", "password123")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if u.Role != "user" {
		t.Fatalf("expected role 'user', got %s", u.Role)
	}

	// 2. 测试认证正确与错误密码
	authed, err := s.Authenticate("alice", "password123")
	if err != nil || authed == nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if _, err := s.Authenticate("alice", "wrongpass"); err == nil {
		t.Fatalf("expected authentication failure for wrong password")
	}

	// 3. 测试创建管理员与列表查询
	_, err = s.CreateUser("bob", "bobpass", "admin")
	if err != nil {
		t.Fatalf("CreateUser admin failed: %v", err)
	}
	list, err := s.ListUsers()
	if err != nil || len(list) < 2 {
		t.Fatalf("expected at least 2 users, got %d (err: %v)", len(list), err)
	}

	// 4. 测试修改角色与切换状态
	if err := s.SetRole("alice", "admin"); err != nil {
		t.Fatalf("SetRole failed: %v", err)
	}
	active, err := s.ToggleActive("alice")
	if err != nil || active {
		t.Fatalf("expected alice to be disabled, got active=%v, err=%v", active, err)
	}
	if _, err := s.Authenticate("alice", "password123"); err != ErrUserDisabled {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}

	// 5. 测试删除用户
	if err := s.DeleteUser("alice"); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}
}

func TestAuthStoreSessions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ab-sess-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	s, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.Now().Truncate(time.Second)
	exp := now.Add(24 * time.Hour)

	// 保存 Session
	if err := s.SaveSession("tok1", "user1", "user", exp); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}
	if err := s.SaveSession("tok2", "user1", "user", exp); err != nil {
		t.Fatalf("SaveSession 2 failed: %v", err)
	}

	active, err := s.GetActiveSessions()
	if err != nil || len(active) != 2 {
		t.Fatalf("expected 2 active sessions, got %d (err: %v)", len(active), err)
	}

	// 删除单 Session
	if err := s.DeleteSession("tok1"); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	active, _ = s.GetActiveSessions()
	if len(active) != 1 || active[0].Token != "tok2" {
		t.Fatalf("expected 1 session with tok2, got %v", active)
	}

	// 按用户名清退所有 Session
	if err := s.DeleteSessionsByUsername("user1"); err != nil {
		t.Fatalf("DeleteSessionsByUsername failed: %v", err)
	}
	active, _ = s.GetActiveSessions()
	if len(active) != 0 {
		t.Fatalf("expected 0 sessions after user eviction, got %d", len(active))
	}
}

func TestAuthStoreAuditLogsAndPrune(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ab-audit-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	s, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// 记录审计
	for i := 0; i < 15; i++ {
		eventType := "LOGIN_FAILED"
		if i%2 == 0 {
			eventType = "USER_LOGIN"
		}
		if err := s.RecordAudit(eventType, "actor1", "target1", "127.0.0.1", "details test"); err != nil {
			t.Fatalf("RecordAudit failed: %v", err)
		}
	}

	// 查询审计日志
	logs, total, err := s.ListAuditLogs(10, 0, "", "")
	if err != nil || total != 15 || len(logs) != 10 {
		t.Fatalf("ListAuditLogs failed: total=%d, count=%d, err=%v", total, len(logs), err)
	}

	// 按 event_type 过滤
	loginLogs, totalLogin, err := s.ListAuditLogs(50, 0, "USER_LOGIN", "")
	if err != nil || totalLogin != 8 || len(loginLogs) != 8 {
		t.Fatalf("ListAuditLogs filter failed: total=%d, count=%d", totalLogin, len(loginLogs))
	}

	// 修剪审计日志，仅保留最新 5 条
	if err := s.PruneAuditLogs(5); err != nil {
		t.Fatalf("PruneAuditLogs failed: %v", err)
	}
	_, prunedTotal, _ := s.ListAuditLogs(50, 0, "", "")
	if prunedTotal != 5 {
		t.Fatalf("expected 5 logs after prune, got %d", prunedTotal)
	}
}

func TestAuthStoreSettings(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ab-settings-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	s, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if v := s.GetSetting("allow_register", "default_val"); v != "default_val" {
		t.Fatalf("expected default_val, got %s", v)
	}

	if err := s.SetSetting("allow_register", "false"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}
	if v := s.GetSetting("allow_register", "true"); v != "false" {
		t.Fatalf("expected false, got %s", v)
	}

	if err := s.SetSetting("max_slots", "25"); err != nil {
		t.Fatalf("SetSetting max_slots failed: %v", err)
	}

	all, err := s.GetAllSettings()
	if err != nil || len(all) < 2 {
		t.Fatalf("GetAllSettings failed: %v, count=%d", err, len(all))
	}
	if all["allow_register"] != "false" || all["max_slots"] != "25" {
		t.Fatalf("unexpected settings: %v", all)
	}
}

func TestCountActiveAdmins(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ab-admins-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	s, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// 初始默认有一个 admin 账号
	cnt, err := s.CountActiveAdmins()
	if err != nil || cnt != 1 {
		t.Fatalf("expected 1 initial admin, got %d (err: %v)", cnt, err)
	}

	// 创建第 2 个 admin
	if _, err := s.CreateUser("admin2", "password123", "admin"); err != nil {
		t.Fatal(err)
	}
	cnt, _ = s.CountActiveAdmins()
	if cnt != 2 {
		t.Fatalf("expected 2 active admins, got %d", cnt)
	}
}

func TestRandomPasswordGenerator(t *testing.T) {
	p1, err := generateRandomPassword(16)
	if err != nil || len(p1) != 16 {
		t.Fatalf("generateRandomPassword failed: %s (err: %v)", p1, err)
	}
	p2, err := generateRandomPassword(16)
	if err != nil || len(p2) != 16 {
		t.Fatalf("generateRandomPassword failed: %s (err: %v)", p2, err)
	}
	if p1 == p2 {
		t.Fatalf("generateRandomPassword collision detected: %s == %s", p1, p2)
	}
}

