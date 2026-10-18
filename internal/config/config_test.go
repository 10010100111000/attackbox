package config

import (
	"testing"
)

func TestConfigDefaultsAndSlots(t *testing.T) {
	c := &Config{
		MaxSlots: 10,
	}
	c.defaults()

	if len(c.Boxes) != 10 {
		t.Fatalf("expected 10 boxes, got %d", len(c.Boxes))
	}
	if c.Listen != ":443" {
		t.Fatalf("expected default listen :443, got %s", c.Listen)
	}

	b1 := c.BoxByID("box1")
	if b1 == nil || b1.Index != 1 || b1.Display != 101 {
		t.Fatalf("invalid box1: %v", b1)
	}

	// 动态扩容到 15 槽位
	c.EnsureSlots(15)
	if len(c.Boxes) != 15 {
		t.Fatalf("expected 15 boxes after expansion, got %d", len(c.Boxes))
	}
	b15 := c.BoxByID("box15")
	if b15 == nil || b15.Index != 15 || b15.Display != 115 {
		t.Fatalf("invalid box15: %v", b15)
	}
}
