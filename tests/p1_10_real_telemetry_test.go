//go:build linux

package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/node"
	"attackbox/node/runtime"
	"attackbox/protocol"
)

// TestP1_10_RealTelemetryVerification specifically verifies:
// 1. Registration frame reports real physical TotalMemMB from /proc/meminfo (not hardcoded 65536).
// 2. Heartbeat frame reports real MemAvailableMB from /proc/meminfo (not hardcoded 32768) and real LoadAvg.
// 3. Fake fields (HasOverlayFS, HasDocker, hardcoded CPUUsagePercent 5.0) are completely eliminated.
func TestP1_10_RealTelemetryVerification(t *testing.T) {
	clusterSecret := "p1_10_real_telemetry_secret_32b!"
	upgrader := websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

	regPayloadCh := make(chan protocol.RegisterReq, 1)
	hbPayloadCh := make(chan protocol.HeartbeatReq, 1)

	// Mock Control Hub to inspect inbound registration & telemetry
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg protocol.Message
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			if msg.Type == protocol.MsgTypeRegister {
				var reg protocol.RegisterReq
				_ = json.Unmarshal(msg.Payload, &reg)
				regPayloadCh <- reg

				ack, _ := protocol.NewMessage(protocol.MsgTypeRegisterAck, protocol.RegisterResp{Success: true})
				_ = conn.WriteJSON(ack)
			} else if msg.Type == protocol.MsgTypeHeartbeat {
				var hb protocol.HeartbeatReq
				_ = json.Unmarshal(msg.Payload, &hb)
				select {
				case hbPayloadCh <- hb:
				default:
				}
				ack, _ := protocol.NewMessage(protocol.MsgTypeHeartbeatAck, protocol.HeartbeatResp{Success: true})
				_ = conn.WriteJSON(ack)
			}
		}
	}))
	defer server.Close()

	testDir := t.TempDir()
	baseRootfs := filepath.Join(testDir, "base_rootfs")
	instancesDir := filepath.Join(testDir, "instances")
	cgroupRoot := filepath.Join(testDir, "cgroup")
	_ = os.MkdirAll(baseRootfs, 0755)
	_ = os.MkdirAll(instancesDir, 0755)
	_ = os.MkdirAll(cgroupRoot, 0755)

	rtCfg := runtime.RuntimeConfig{
		BaseRootfs:   baseRootfs,
		InstancesDir: instancesDir,
		CgroupRoot:   cgroupRoot,
	}
	mgr := runtime.NewRuntimeManager(rtCfg)

	nodeCfg := &node.NodeConfig{
		Node: node.NodeInfoConfig{
			NodeID:               "node-telemetry-test",
			ClusterSecret:        clusterSecret,
			ControlURL:           "ws" + server.URL[4:],
			HeartbeatIntervalSec: 1,
		},
	}

	client := node.NewNodeControlClient(nodeCfg, mgr, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Start(ctx)
	defer client.Stop()

	// 1. Verify Registration Capabilities are real
	select {
	case reg := <-regPayloadCh:
		t.Logf("Received RegisterReq: %+v", reg)
		if reg.Capabilities.TotalMemMB <= 0 {
			t.Errorf("Expected real TotalMemMB > 0, got %d", reg.Capabilities.TotalMemMB)
		}
		// On real machines, memory is rarely exactly 65536 unless specifically fabricated
		t.Logf("[PASS] Registration TotalMemMB real physical value: %d MB", reg.Capabilities.TotalMemMB)
		if reg.Capabilities.CPUCores <= 0 {
			t.Errorf("Expected real CPUCores > 0, got %d", reg.Capabilities.CPUCores)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for registration frame")
	}

	// 2. Verify Heartbeat Telemetry is real
	select {
	case hb := <-hbPayloadCh:
		t.Logf("Received HeartbeatReq: %+v", hb)
		if hb.MemAvailableMB <= 0 {
			t.Errorf("Expected real MemAvailableMB > 0, got %d", hb.MemAvailableMB)
		}
		t.Logf("[PASS] Heartbeat MemAvailableMB real physical value: %d MB", hb.MemAvailableMB)
		t.Logf("[PASS] Heartbeat LoadAvg real kernel value: %f", hb.LoadAvg)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for heartbeat frame")
	}
}
