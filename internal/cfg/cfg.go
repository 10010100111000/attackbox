// Package cfg loads the on-disk configuration.
package cfg

import (
	"encoding/json"
	"fmt"
	"os"
)

// Box describes one sandbox account.
type Box struct {
	ID            string `json:"id"`
	User          string `json:"user"`
	UID           int    `json:"uid"`
	GID           int    `json:"gid"`
	Home          string `json:"home"`
	RunDir        string `json:"runDir"`
	GuiSocket     string `json:"guiSocket"`
	TmuxSocket    string `json:"tmuxSocket"`
	Display       int    `json:"display"`
	Geometry      string `json:"geometry"`
	SessionUnit   string `json:"sessionUnit"`
	SessionScript string `json:"sessionScript"`
	MemoryMax     string `json:"memoryMax"`
	CPUQuota      string `json:"cpuQuota"`
	TTLMinutes    int    `json:"ttlMinutes"`
}

// User is one login.
type User struct {
	Name  string `json:"name"`
	Box   string `json:"box"`
	Token string `json:"token"`
}

type Config struct {
	Listen   string `json:"listen"`
	Cert     string `json:"cert"`
	Key      string `json:"key"`
	NovncDir string `json:"novncDir"`
	XtermDir string `json:"xtermDir"`
	Boxes    []Box  `json:"boxes"`
	Users    []User `json:"users"`
}

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
	if c.NovncDir == "" {
		c.NovncDir = "/usr/share/novnc"
	}
	if c.XtermDir == "" {
		c.XtermDir = "/usr/local/share/attackbox/xterm"
	}
	for i := range c.Boxes {
		b := &c.Boxes[i]
		if b.Home == "" {
			b.Home = "/home/" + b.User
		}
		if b.RunDir == "" {
			b.RunDir = "/run/attackbox/" + b.ID
		}
		if b.GuiSocket == "" {
			b.GuiSocket = b.RunDir + "/gui.sock"
		}
		if b.TmuxSocket == "" {
			b.TmuxSocket = b.RunDir + "/tmux.sock"
		}
		if b.SessionUnit == "" {
			b.SessionUnit = "attackbox-" + b.ID + "-gui"
		}
		if b.SessionScript == "" {
			b.SessionScript = "/usr/local/lib/attackbox/box-launch.sh"
		}
		if b.Geometry == "" {
			b.Geometry = "1280x800"
		}
		if b.MemoryMax == "" {
			b.MemoryMax = "2G"
		}
		if b.CPUQuota == "" {
			b.CPUQuota = "150%"
		}
	}
}

func (c *Config) BoxByID(id string) *Box {
	for i := range c.Boxes {
		if c.Boxes[i].ID == id {
			return &c.Boxes[i]
		}
	}
	return nil
}
