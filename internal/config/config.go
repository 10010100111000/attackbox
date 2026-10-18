// Package config loads and validates the attackbox service configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Box defines one isolated sandbox slot.
type Box struct {
	ID         string `json:"id"`
	User       string `json:"user"`
	Display    int    `json:"display"`
	RunDir     string `json:"runDir"`
	GuiSocket  string `json:"guiSocket"`
	TTLMinutes int    `json:"ttlMinutes"`
	Index      int    `json:"index"`
}

// Config defines the global server configuration.
type Config struct {
	Listen      string `json:"listen"`
	Cert        string `json:"cert"`
	Key         string `json:"key"`
	WebDir      string `json:"webDir"`
	NovncDir    string `json:"novncDir"`
	XtermDir    string `json:"xtermDir"`
	BaseRootfs  string `json:"baseRootfs"`
	BoxesDir    string `json:"boxesDir"`
	MaxSlots    int    `json:"maxSlots"`
	BaseDisplay int    `json:"baseDisplay"`
	SandboxUser string `json:"sandboxUser"`
	TTLMinutes         int    `json:"ttlMinutes"`
	IdleTimeoutMinutes int    `json:"idleTimeoutMinutes"`
	TrustProxy         bool   `json:"trustProxy"`
	Boxes       []Box  `json:"boxes"`
}

// Load reads and parses configuration from a JSON file on disk.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.defaults()
	return &c, nil
}

func (c *Config) defaults() {
	if c.Listen == "" {
		c.Listen = ":443"
	}
	if c.BaseRootfs == "" {
		c.BaseRootfs = "/var/lib/attackbox/base_rootfs"
	}
	if c.BoxesDir == "" {
		c.BoxesDir = "/var/lib/attackbox/boxes"
	}
	if c.WebDir == "" {
		if _, err := os.Stat("./web"); err == nil {
			c.WebDir = "./web"
		} else if exe, err := os.Executable(); err == nil && fileExists(filepath.Join(filepath.Dir(exe), "web")) {
			c.WebDir = filepath.Join(filepath.Dir(exe), "web")
		} else {
			c.WebDir = "/usr/local/share/attackbox/web"
		}
	}
	if c.NovncDir == "" {
		c.NovncDir = "/usr/share/novnc"
	}
	if c.XtermDir == "" {
		c.XtermDir = "/usr/local/share/attackbox/xterm"
	}
	if c.MaxSlots <= 0 {
		if len(c.Boxes) > 0 {
			c.MaxSlots = len(c.Boxes)
		} else {
			c.MaxSlots = 20
		}
	}
	if c.BaseDisplay <= 0 {
		c.BaseDisplay = 100
	}
	if c.SandboxUser == "" {
		c.SandboxUser = "box"
	}
	if c.TTLMinutes <= 0 {
		c.TTLMinutes = 120
	}
	if c.IdleTimeoutMinutes <= 0 {
		c.IdleTimeoutMinutes = 10
	}

	// 动态由 MaxSlots 派生所有沙箱槽位 (兼容静态 Boxes 配置)
	if len(c.Boxes) == 0 {
		c.Boxes = make([]Box, c.MaxSlots)
		for i := 0; i < c.MaxSlots; i++ {
			idx := i + 1
			c.Boxes[i] = Box{
				ID:         fmt.Sprintf("box%d", idx),
				User:       c.SandboxUser,
				Display:    c.BaseDisplay + idx,
				RunDir:     fmt.Sprintf("/run/attackbox/box%d", idx),
				GuiSocket:  fmt.Sprintf("/run/attackbox/box%d/gui.sock", idx),
				TTLMinutes: c.TTLMinutes,
				Index:      idx,
			}
		}
	} else {
		for i := range c.Boxes {
			b := &c.Boxes[i]
			if b.Index <= 0 {
				b.Index = i + 1
			}
			if b.User == "" {
				b.User = c.SandboxUser
			}
			if b.Display <= 0 {
				b.Display = c.BaseDisplay + b.Index
			}
			if b.RunDir == "" {
				b.RunDir = fmt.Sprintf("/run/attackbox/%s", b.ID)
			}
			if b.GuiSocket == "" {
				b.GuiSocket = b.RunDir + "/gui.sock"
			}
			if b.TTLMinutes <= 0 {
				b.TTLMinutes = c.TTLMinutes
			}
		}
		c.EnsureSlots(c.MaxSlots)
	}
}

// EnsureSlots dynamically expands the Boxes slice if the requested slots exceed current length.
func (c *Config) EnsureSlots(slots int) {
	if slots <= len(c.Boxes) {
		return
	}
	for i := len(c.Boxes); i < slots; i++ {
		idx := i + 1
		c.Boxes = append(c.Boxes, Box{
			ID:         fmt.Sprintf("box%d", idx),
			User:       c.SandboxUser,
			Display:    c.BaseDisplay + idx,
			RunDir:     fmt.Sprintf("/run/attackbox/box%d", idx),
			GuiSocket:  fmt.Sprintf("/run/attackbox/box%d/gui.sock", idx),
			TTLMinutes: c.TTLMinutes,
			Index:      idx,
		})
	}
}

// BoxByID returns the box with the given ID, or nil if not found.
func (c *Config) BoxByID(id string) *Box {
	for i := range c.Boxes {
		if c.Boxes[i].ID == id {
			return &c.Boxes[i]
		}
	}
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
