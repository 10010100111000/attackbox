package control

import (
	"attackbox/protocol"
	"time"
)

// A complete inventory is authoritative only after the worker's acknowledged
// creation revision. Unknown instances after a Control restart are reset.
func (s *ControlHTTPServer) syncWorkerState(workerID string, hb protocol.HeartbeatReq) {
	if !s.clusterMgr.IsCurrentInventory(workerID, hb) {
		return
	}
	alive := make(map[string]protocol.SandboxSummary, len(hb.Sandboxes))
	for _, item := range hb.Sandboxes {
		alive[item.ID] = item
	}
	ready := true
	for _, sess := range s.sessions.ListAll() {
		if sess.WorkerNodeID != workerID {
			continue
		}
		if sess.WorkerBootID != "" && sess.WorkerBootID != hb.BootID {
			s.purgeSessionLocally(sess.SandboxID)
			continue
		}
		item, present := alive[sess.SandboxID]
		delete(alive, sess.SandboxID)
		if sess.State == SessionStateCreating || sess.State == SessionStateTerminating {
			continue
		}
		if !present && sess.InventoryRevision > 0 && hb.InventoryRevision >= sess.InventoryRevision {
			s.purgeSessionLocally(sess.SandboxID)
			continue
		}
		if sess.State == SessionStateCleanupFailed || (present && item.State == "cleanup_failed") {
			s.asyncCompensatingDestroy(workerID, sess.SandboxID, "retry_cleanup")
		} else if present {
			if item.LastActiveAt > 0 {
				s.sessions.Touch(sess.SandboxID, time.Unix(item.LastActiveAt, 0))
			}
			s.sessions.SetGUIRunning(sess.SandboxID, item.GUIRunning)
		}
	}
	for id := range alive {
		ready = false
		s.asyncCompensatingDestroy(workerID, id, "unknown_after_control_restart")
	}
	s.clusterMgr.SetInventoryReady(workerID, hb.BootID, hb.Seq, ready)
}
