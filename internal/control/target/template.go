package target

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"

	"attackbox/pkg/protocol"
)

// LabTemplate represents a declarative target and sandbox specification.
type LabTemplate struct {
	LabID             string                 `yaml:"lab_id" json:"lab_id"`
	Name              string                 `yaml:"name" json:"name"`
	Category          string                 `yaml:"category" json:"category"`
	Difficulty        string                 `yaml:"difficulty" json:"difficulty"`
	PointsCostPerHour int                    `yaml:"points_cost_per_hour" json:"points_cost_per_hour"`
	Description       string                 `yaml:"description" json:"description"`
	SandboxPolicy     protocol.SandboxPolicy `yaml:"sandbox_policy" json:"sandbox_policy"`
	Target            protocol.TargetSpec    `yaml:"target" json:"target"`
}

// TemplateManager loads and indexes lab templates.
type TemplateManager struct {
	mu        sync.RWMutex
	templates map[string]*LabTemplate
}

// NewTemplateManager creates an empty template registry.
func NewTemplateManager() *TemplateManager {
	return &TemplateManager{
		templates: make(map[string]*LabTemplate),
	}
}

// LoadFromDir scans the given directory for *.yaml template definitions.
func (m *TemplateManager) LoadFromDir(dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read template dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || (!filepath.HasPrefix(entry.Name(), ".") && (filepath.Ext(entry.Name()) == ".yaml" || filepath.Ext(entry.Name()) == ".yml")) {
			fullPath := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}

			var tpl LabTemplate
			if err := yaml.Unmarshal(data, &tpl); err != nil {
				continue
			}

			if tpl.LabID != "" {
				m.templates[tpl.LabID] = &tpl
			}
		}
	}

	return nil
}

// Get returns the template by its unique lab_id.
func (m *TemplateManager) Get(labID string) (*LabTemplate, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tpl, ok := m.templates[labID]
	return tpl, ok
}

// List returns a slice of all indexed lab templates.
func (m *TemplateManager) List() []*LabTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*LabTemplate, 0, len(m.templates))
	for _, tpl := range m.templates {
		list = append(list, tpl)
	}
	return list
}
