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
	Display    int    `json:"display"`
	RunDir     string `json:"runDir"`
	GuiSocket  string `json:"guiSocket"`
	TTLMinutes int    `json:"ttlMinutes"`
}

// Config defines the global server configuration.
type Config struct {
	Listen   string `json:"listen"`
	Cert     string `json:"cert"`
	Key      string `json:"key"`
	WebDir   string `json:"webDir"`
	NovncDir string `json:"novncDir"`
	XtermDir string `json:"xtermDir"`
	Boxes    []Box  `json:"boxes"`
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
	for i := range c.Boxes {
		b := &c.Boxes[i]
		if b.RunDir == "" {
			b.RunDir = "/run/attackbox/" + b.ID
		}
		if b.GuiSocket == "" {
			b.GuiSocket = b.RunDir + "/gui.sock"
		}
		if b.TTLMinutes <= 0 {
			b.TTLMinutes = 120
		}
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
