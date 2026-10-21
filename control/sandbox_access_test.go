package control

import (
	"attackbox/protocol"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCLIAndGUIShareSandboxPerUser(t *testing.T) {
	f := newControlFixture(t, true)
	w := f.connectWorker(t, "boot")
	var creates atomic.Int32
	w.mu.Lock()
	w.create = func(req protocol.CreateBoxReq) protocol.CreateBoxResp {
		creates.Add(1)
		return protocol.CreateBoxResp{Success: true, SandboxID: req.SandboxID, BootID: "boot", InventoryRevision: 2, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	}
	w.mu.Unlock()
	w.heartbeat(t, 1, 0)
	users := make([]*User, 2)
	for i, name := range []string{"alice", "bob"} {
		var err error
		users[i], err = f.server.db.CreateUser(name, "unused", name+"@example.test", "user")
		if err != nil {
			t.Fatal(err)
		}
	}
	request := func(user *User, method, path, body string) *httptest.ResponseRecorder {
		token, err := SignJWT(f.private, JWTClaims{Sub: user.ID, Username: user.Username, Role: user.Role, TokenVersion: user.TokenVersion, Exp: time.Now().Add(time.Hour).Unix()})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+token)
		out := httptest.NewRecorder()
		f.server.ServeHTTP(out, req)
		return out
	}
	ids := make([]string, 2)
	for i, user := range users {
		// Cover both orders: CLI then GUI, GUI then CLI.
		paths := []string{"/api/sandbox/start", "/api/env/gui/toggle"}
		if i == 1 {
			paths[0], paths[1] = paths[1], paths[0]
		}
		for j, path := range paths {
			out := request(user, "POST", path, "{}")
			expected := 201
			if j == 1 {
				expected = 200
			}
			if out.Code != expected {
				t.Fatalf("%s: %d %s", path, out.Code, out.Body.String())
			}
			var resp StartSandboxResp
			if err := json.Unmarshal(out.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if j == 0 {
				ids[i] = resp.SandboxID
			} else if ids[i] != resp.SandboxID {
				t.Fatal("CLI and GUI allocated different sandboxes")
			}
			for typ, raw := range map[string]string{"pty": resp.Ticket, "vnc": resp.GUITicket} {
				claims, err := protocol.VerifyTicket(raw, f.server.pubKey)
				if err != nil {
					t.Fatal(err)
				}
				if claims.Sub != user.ID || claims.Sbx != ids[i] || claims.Typ != typ {
					t.Fatalf("incorrect ticket ownership: %+v", claims)
				}
			}
		}
	}
	if creates.Load() != 2 || ids[0] == ids[1] {
		t.Fatalf("incorrect per-user allocations: count=%d ids=%v", creates.Load(), ids)
	}
	out := request(users[1], "POST", "/api/sandbox/stop", `{"sandbox_id":"`+ids[0]+`"}`)
	if out.Code != 403 {
		t.Fatalf("another user can stop sandbox: %d %s", out.Code, out.Body.String())
	}
	out = request(users[0], "POST", "/api/sandbox/stop", `{"sandbox_id":"`+ids[0]+`"}`)
	if out.Code != 200 {
		t.Fatalf("owner cannot stop sandbox: %d %s", out.Code, out.Body.String())
	}
	if _, ok := f.server.sessions.Get(ids[1]); !ok {
		t.Fatal("stopping one user removed another user's sandbox")
	}
}
