package box

import (
	"testing"
	"time"
)

func TestPoolAllocation(t *testing.T) {
	pool := NewPool([]string{"box1", "box2"})
	slot, err := pool.Acquire("alice")
	if err != nil || slot.ID != "box1" {
		t.Fatalf("expected box1, got %v, err %v", slot, err)
	}
	// 重复获取应返回同一槽位
	slot2, _ := pool.Acquire("alice")
	if slot2.ID != "box1" {
		t.Fatalf("expected alice to keep box1")
	}

	slotBob, err := pool.Acquire("bob", 10*time.Minute)
	if err != nil || slotBob.ID != "box2" {
		t.Fatalf("expected box2 for bob, got %v, err %v", slotBob, err)
	}

	// 槽位已满
	_, err = pool.Acquire("charlie")
	if err == nil {
		t.Fatalf("expected error when pool full")
	}

	// 释放 alice
	_, err = pool.Release("alice")
	if err != nil {
		t.Fatalf("expected release success: %v", err)
	}

	// charlie 现在应能获取 box1
	slotCharlie, err := pool.Acquire("charlie")
	if err != nil || slotCharlie.ID != "box1" {
		t.Fatalf("expected box1 for charlie, got %v, err %v", slotCharlie, err)
	}
}
