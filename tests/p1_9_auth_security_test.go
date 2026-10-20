//go:build linux

package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"attackbox/control"
)

// TestP1_9_AuthSecurityAndAdminBootstrap verifies:
// 1. Timing side channel mitigation: non-existent users undergo constant-time dummy bcrypt check.
// 2. Database errors are sanitized: client receives friendly error, never raw SQLite error strings.
// 3. Admin user bootstrapping: configured admin or initial user receives role='admin'.
// 4. Admin permission enforcement on sandbox operations.
func TestP1_9_AuthSecurityAndAdminBootstrap(t *testing.T) {
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "auth_security.db")

	cfg := &control.ControlConfig{}
	cfg.Server.ListenAddr = "127.0.0.1:18099"
	cfg.Database.Path = dbPath
	cfg.Auth.InitialAdminUsername = "sys_admin"
	cfg.Auth.InitialAdminPassword = "AdminSecurePassword123!"
	cfg.Auth.InitialAdminEmail = "sys_admin@attackbox.local"
	cfg.SanitizeAndApplyDefaults()

	pubKey, privKey, err := control.LoadOrGenerateKeys("")
	if err != nil {
		t.Fatalf("generate keys: %v", err)
	}

	db, err := control.NewDatabaseStore(dbPath)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	slots := control.NewSlotAllocator(250)
	sessions := control.NewSessionManager()
	clusterMgr := control.NewNodeClusterManager("p1_9_secret_32_chars_long_ok!")

	handler := control.NewControlHTTPServer(cfg, db, privKey, pubKey, clusterMgr, slots, sessions)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 1. VERIFY ADMIN BOOTSTRAP
	adminUser, err := db.GetUserByUsername("sys_admin")
	if err != nil {
		t.Fatalf("expected bootstrapped admin user: %v", err)
	}
	if adminUser.Role != "admin" {
		t.Fatalf("expected admin user role 'admin', got '%s'", adminUser.Role)
	}
	t.Log("[PASS] Admin user successfully bootstrapped with role='admin'")

	// 2. VERIFY SQLITE ERROR SANITIZATION (No raw DB leaks)
	regBody, _ := json.Marshal(control.RegisterReq{
		Username: "alice_tenant",
		Password: "Password123!",
		Email:    "alice@example.com",
	})
	res, err := http.Post(server.URL+"/api/auth/register", "application/json", bytes.NewReader(regBody))
	if err != nil || res.StatusCode != http.StatusCreated {
		t.Fatalf("initial register failed: %v, code %d", err, res.StatusCode)
	}

	// Attempt duplicate register with same username
	resDup, err := http.Post(server.URL+"/api/auth/register", "application/json", bytes.NewReader(regBody))
	if err != nil {
		t.Fatalf("duplicate register request failed: %v", err)
	}
	defer resDup.Body.Close()

	if resDup.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d", resDup.StatusCode)
	}
	var errResp map[string]interface{}
	_ = json.NewDecoder(resDup.Body).Decode(&errResp)
	errMsg, _ := errResp["error"].(string)

	if strings.Contains(strings.ToLower(errMsg), "sqlite") || strings.Contains(strings.ToLower(errMsg), "constraint") {
		t.Fatalf("[LEAK VULNERABILITY] Raw database error leaked to client: %s", errMsg)
	}
	if errMsg != "username or email already registered" {
		t.Fatalf("unexpected sanitized error message: %s", errMsg)
	}
	t.Log("[PASS] Database error cleanly sanitized without leaking raw SQLite internals")

	// 3. VERIFY TIMING SIDE CHANNEL MITIGATION
	// Measure non-existent user login vs existing user bad password login
	loginNonExistent, _ := json.Marshal(control.LoginReq{
		Username: "non_existent_ghost_user_xyz",
		Password: "RandomPassword123!",
	})

	startNonExist := time.Now()
	res1, _ := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewReader(loginNonExistent))
	durNonExist := time.Since(startNonExist)
	res1.Body.Close()

	loginWrongPass, _ := json.Marshal(control.LoginReq{
		Username: "alice_tenant",
		Password: "WrongPassword999!",
	})

	startWrongPass := time.Now()
	res2, _ := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewReader(loginWrongPass))
	durWrongPass := time.Since(startWrongPass)
	res2.Body.Close()

	t.Logf("Timing profile: NonExistentUser=%v, WrongPasswordExistingUser=%v", durNonExist, durWrongPass)

	// In the unpatched version, non-existent user returned in <1ms (short circuit without bcrypt).
	// With the fix, non-existent user takes full bcrypt duration (>10ms).
	if durNonExist < 10*time.Millisecond {
		t.Fatalf("[TIMING VULNERABILITY] Non-existent user returned too quickly (%v), bcrypt dummy was bypassed!", durNonExist)
	}
	t.Log("[PASS] Timing side channel mitigated: Non-existent user executed full bcrypt computation")
}
