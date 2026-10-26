package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A read-only retirement gate, not an old runtime. Never erase cleanup duties
// merely because the executor has been removed. Custom old state locations must
// also be checked by the operator before removing the retired configuration.
func checkRetiredTargets(instances string) error {
	if instances == "" {
		instances = "/var/lib/attackbox/instances"
	}
	path := filepath.Join(filepath.Dir(filepath.Clean(instances)), "docker-target-state")
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return fmt.Errorf("invalid retired target ledger; finish cleanup with the previous Worker before upgrading")
		}
		b, err := os.ReadFile(filepath.Join(path, entry.Name()))
		if err != nil {
			return err
		}
		var record struct {
			Cleaned bool `json:"cleaned"`
		}
		if json.Unmarshal(b, &record) != nil || !record.Cleaned {
			return fmt.Errorf("retired Docker targets have unconfirmed cleanup; release them using the previous Worker before upgrading")
		}
	}
	return nil
}
