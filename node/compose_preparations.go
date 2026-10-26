package node

import (
	"context"
	"sync"
	"time"

	"attackbox/node/labcompose"
	"attackbox/protocol"
)

type composeJob struct {
	retainedAt                                       time.Time
	Logs                                             []string
	State, DeploymentID, Error, Fingerprint, OwnerID string
	Summary                                          *protocol.ComposeDefinition
	cancel                                           context.CancelFunc
}
type composePreparations struct {
	mu     sync.Mutex
	jobs   map[string]composeJob
	slot   chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	work   func(context.Context, protocol.ComposeRequest) (labcompose.Deployment, error)
	lookup func(context.Context, string) (labcompose.Deployment, error)
}

func newComposePreparations(parent context.Context, work func(context.Context, protocol.ComposeRequest) (labcompose.Deployment, error), lookup func(context.Context, string) (labcompose.Deployment, error)) *composePreparations {
	ctx, cancel := context.WithCancel(parent)
	return &composePreparations{jobs: map[string]composeJob{}, slot: make(chan struct{}, 1), ctx: ctx, cancel: cancel, work: work, lookup: lookup}
}
func (p *composePreparations) Close() { p.mu.Lock(); p.cancel(); p.mu.Unlock(); p.wg.Wait() }
func (p *composePreparations) handle(req protocol.ComposeRequest, action string) composeJob {
	if !bundleIDPattern.MatchString(req.PreparationID) || req.TargetID != req.PreparationID {
		return composeJob{State: "failed", Error: "invalid_preparation_identity"}
	}
	p.mu.Lock()
	old, exists := p.jobs[req.PreparationID]
	if exists && old.OwnerID != req.OwnerID {
		p.mu.Unlock()
		return composeJob{State: "failed", Error: "preparation_identity_changed"}
	}
	if action == protocol.MsgComposePrepareStatus {
		p.mu.Unlock()
		if exists {
			return old
		}
		if req.DeploymentID != "" {
			if d, err := p.lookup(p.ctx, req.DeploymentID); err == nil {
				return composeJob{State: "unknown", DeploymentID: d.ID, Summary: definitionSummary(d), Logs: []string{"未找到当前任务；原部署产物已核验"}}
			}
		}
		return composeJob{State: "unknown", Logs: []string{"节点无此准备任务，请取消后重新准备"}}
	}
	if action == protocol.MsgComposePrepareCancel {
		if exists && old.State == "preparing" {
			old.State = "cancelling"
			old.Logs = append(old.Logs, "已请求取消，等待构建或下载退出")
			old.cancel()
			p.jobs[req.PreparationID] = old
		} else if !exists {
			if !p.trim() {
				p.mu.Unlock()
				return composeJob{State: "unknown", Error: "compose_prepare_busy"}
			}
			old = composeJob{retainedAt: time.Now(), OwnerID: req.OwnerID, State: "cancelled", Logs: []string{"准备任务已取消"}}
			p.jobs[req.PreparationID] = old
		}
		p.mu.Unlock()
		return old
	}
	if exists {
		if old.State == "cancelled" || old.State == "cancelling" {
			p.mu.Unlock()
			return old
		}
		if old.Fingerprint != composeRequestFingerprint(req) {
			p.mu.Unlock()
			return composeJob{State: "failed", Error: "preparation_identity_changed"}
		}
		// Attempts are immutable. Failed attempts require a new ID, just like cancelled ones.
		p.mu.Unlock()
		return old
	}
	age := time.Since(req.RequestedAt)
	if req.RequestedAt.IsZero() || age > 2*time.Minute || age < -30*time.Second {
		p.mu.Unlock()
		return composeJob{State: "failed", Error: "preparation_request_expired"}
	}
	if !p.trim() {
		p.mu.Unlock()
		return composeJob{State: "failed", Error: "compose_prepare_busy"}
	}
	select {
	case <-p.ctx.Done():
		p.mu.Unlock()
		return composeJob{State: "failed", Error: "worker_stopping"}
	default:
	}
	select {
	case p.slot <- struct{}{}:
	default:
		p.mu.Unlock()
		return composeJob{State: "failed", Error: "compose_prepare_busy"}
	}
	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Minute)
	job := composeJob{retainedAt: time.Now(), State: "preparing", OwnerID: req.OwnerID, Fingerprint: composeRequestFingerprint(req), cancel: cancel, Logs: []string{"已接收准备任务", "正在下载、校验与准备镜像"}}
	if req.Rebuild {
		job.Logs = append(job.Logs, "强制重建：刷新镜像并跳过构建缓存")
	}
	p.jobs[req.PreparationID] = job
	p.wg.Add(1)
	p.mu.Unlock()
	go func() {
		defer p.wg.Done()
		defer cancel()
		var d labcompose.Deployment
		err := ctx.Err()
		if err == nil {
			d, err = p.work(ctx, req)
		}
		// Work (including temporary file cleanup) has exited before terminal status is visible.
		p.mu.Lock()
		defer p.mu.Unlock()
		<-p.slot
		result := p.jobs[req.PreparationID]
		result.cancel = nil
		result.retainedAt = time.Now()
		switch {
		case result.State == "cancelling" || ctx.Err() == context.Canceled:
			result.State = "cancelled"
			result.Error = ""
			result.Logs = append(result.Logs, "准备任务已取消")
		case ctx.Err() == context.DeadlineExceeded:
			result.State = "failed"
			result.Error = "compose_prepare_timeout"
			result.Logs = append(result.Logs, "准备超过 30 分钟，已终止")
		case err != nil:
			result.State = "failed"
			result.Error = "compose_prepare_failed"
			result.Logs = append(result.Logs, composeDiagnostic(err))
		default:
			result.State = "ready"
			result.DeploymentID = d.ID
			result.Summary = definitionSummary(d)
			result.Logs = append(result.Logs, "Compose 校验与镜像准备完成", "已保存不可变部署产物")
		}
		p.jobs[req.PreparationID] = result
	}()
	return job
}

// Retain cancellation intent longer than the maximum admission age plus clock
// tolerance. Expired wire requests cannot resurrect an evicted attempt. No timer.
func (p *composePreparations) trim() bool {
	if len(p.jobs) < 256 {
		return true
	}
	for id, j := range p.jobs {
		if j.State != "preparing" && j.State != "cancelling" && time.Since(j.retainedAt) > 3*time.Minute {
			delete(p.jobs, id)
			return true
		}
	}
	return false
}
