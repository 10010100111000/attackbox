package box

import (
	"fmt"
	"sync"
	"time"
)

type SlotState string

const (
	StateIdle  SlotState = "idle"
	StateInUse SlotState = "in_use"
	StateClean SlotState = "cleaning"
)

type Slot struct {
	ID        string    `json:"id"`
	Index     int       `json:"index"`
	User      string    `json:"user"`
	State     SlotState `json:"state"`
	ExpiresAt time.Time `json:"expires_at"`
}

type SlotStatus struct {
	ID        string    `json:"id"`
	Index     int       `json:"index"`
	User      string    `json:"user"`
	State     SlotState `json:"state"`
	Remaining string    `json:"remaining"`
}

type Pool struct {
	mu    sync.Mutex
	slots []*Slot
}

func NewPool(ids []string) *Pool {
	p := &Pool{}
	for i, id := range ids {
		p.slots = append(p.slots, &Slot{
			ID:    id,
			Index: i + 1,
			State: StateIdle,
		})
	}
	return p
}

func (p *Pool) Acquire(user string, ttl ...time.Duration) (*Slot, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	duration := 8 * time.Hour
	if len(ttl) > 0 && ttl[0] > 0 {
		duration = ttl[0]
	}

	// 已有租约则刷新
	for _, s := range p.slots {
		if s.User == user && s.State == StateInUse {
			s.ExpiresAt = time.Now().Add(duration)
			return s, nil
		}
	}

	// 认领空闲槽位
	for _, s := range p.slots {
		if s.State == StateIdle {
			s.State = StateInUse
			s.User = user
			s.ExpiresAt = time.Now().Add(duration)
			return s, nil
		}
	}
	return nil, fmt.Errorf("no idle boxes available in pool")
}

func (p *Pool) Release(user string) (*Slot, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, s := range p.slots {
		if s.User == user {
			s.State = StateIdle
			s.User = ""
			s.ExpiresAt = time.Time{}
			return s, nil
		}
	}
	return nil, fmt.Errorf("no slot found for user %s", user)
}

func (p *Pool) SlotFor(user string) *Slot {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.slots {
		if s.User == user && s.State == StateInUse {
			return s
		}
	}
	return nil
}

func (p *Pool) SlotByID(id string) *Slot {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.slots {
		if s.ID == id {
			return s
		}
	}
	return nil
}

func (p *Pool) Status() []SlotStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	res := make([]SlotStatus, 0, len(p.slots))
	for _, s := range p.slots {
		rem := "-"
		if s.State == StateInUse && !s.ExpiresAt.IsZero() {
			if now.Before(s.ExpiresAt) {
				rem = s.ExpiresAt.Sub(now).Truncate(time.Second).String()
			} else {
				rem = "expired"
			}
		}
		res = append(res, SlotStatus{
			ID:        s.ID,
			Index:     s.Index,
			User:      s.User,
			State:     s.State,
			Remaining: rem,
		})
	}
	return res
}

func (p *Pool) ExpiredUsers() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	var expired []string
	for _, s := range p.slots {
		if s.State == StateInUse && !s.ExpiresAt.IsZero() && now.After(s.ExpiresAt) {
			expired = append(expired, s.User)
		}
	}
	return expired
}
