package auth

import (
	"os"
	"path/filepath"
	"testing"
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
