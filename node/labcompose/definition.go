package labcompose

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gopkg.in/yaml.v3"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)
var deploymentPattern = regexp.MustCompile(`^dep_[a-f0-9]{64}$`)
var imageIDPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func defaults(b Budget) Budget {
	if b.MemoryBytes == 0 {
		b.MemoryBytes = 1 << 30
	}
	if b.NanoCPUs == 0 {
		b.NanoCPUs = 2e9
	}
	if b.PIDs == 0 {
		b.PIDs = 256
	}
	if b.MaxServices == 0 {
		b.MaxServices = 8
	}
	return b
}
func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func text(v any) string           { s, _ := v.(string); return s }

func allowed(m map[string]any, fields string) error {
	set := " " + fields + " "
	for k := range m {
		if !strings.Contains(set, " "+k+" ") {
			return fmt.Errorf("unsupported Compose field: %s", k)
		}
	}
	return nil
}

// Check every component, including package root. Bind sources never escape the
// package or traverse symlinks, even when Compose has resolved them to abs paths.
func safePath(root, path string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("package root must be absolute")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("package path escapes root")
	}
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil {
			return "", fmt.Errorf("package file missing")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("package symlink not supported")
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	return path, nil
}

func readBounded(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > max {
		return nil, fmt.Errorf("package file must be regular and within size limit")
	}
	return io.ReadAll(io.LimitReader(f, max+1))
}

func strictYAML(b []byte) (map[string]any, error) {
	var node yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&node); err != nil {
		return nil, fmt.Errorf("invalid Compose YAML")
	}
	var tail yaml.Node
	if err := dec.Decode(&tail); err != io.EOF {
		return nil, fmt.Errorf("one YAML document required")
	}
	count := 0
	var walk func(*yaml.Node, int) error
	walk = func(n *yaml.Node, depth int) error {
		count++
		if depth > 40 || count > 20000 || n.Kind == yaml.AliasNode {
			return fmt.Errorf("Compose document too complex or contains YAML aliases")
		}
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				k := n.Content[i]
				if k.Kind != yaml.ScalarNode || seen[k.Value] {
					return fmt.Errorf("duplicate or complex YAML key")
				}
				seen[k.Value] = true
			}
		}
		for _, c := range n.Content {
			if err := walk(c, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(&node, 0); err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := node.Decode(&raw); err != nil || raw == nil {
		return nil, fmt.Errorf("Compose root must be an object")
	}
	return raw, nil
}

func Parse(ctx context.Context, bundle Bundle, budget Budget) (Definition, error) {
	var def Definition
	budget = defaults(budget)
	if budget.MaxServices < 1 || budget.MaxServices > 16 || budget.MemoryBytes < 16<<20 || budget.NanoCPUs < 1e6 || budget.PIDs < 16 {
		return def, fmt.Errorf("invalid project budget")
	}
	file, err := safePath(bundle.Directory, bundle.File)
	if err != nil {
		return def, err
	}
	data, err := readBounded(file, 1<<20)
	if err != nil {
		return def, err
	}
	raw, err := strictYAML(data)
	if err != nil {
		return def, err
	}
	if err := allowed(raw, "name version services networks volumes"); err != nil {
		return def, err
	}
	services := object(raw["services"])
	if len(services) == 0 || len(services) > budget.MaxServices {
		return def, fmt.Errorf("service count exceeds project budget")
	}
	for name, value := range services {
		if !namePattern.MatchString(name) {
			return def, fmt.Errorf("invalid service name")
		}
		s := object(value)
		if s == nil {
			return def, fmt.Errorf("invalid service configuration")
		}
		if err := allowed(s, "image command entrypoint environment depends_on healthcheck networks volumes working_dir user init read_only hostname container_name expose restart mem_limit cpus pids_limit stop_signal stop_grace_period"); err != nil {
			return def, err
		}
		if text(s["image"]) == "" {
			return def, fmt.Errorf("every service requires an image; build is not supported yet")
		}
	}
	for name, value := range object(raw["networks"]) {
		if !namePattern.MatchString(name) {
			return def, fmt.Errorf("invalid network key")
		}
		if value != nil {
			if err := allowed(object(value), "driver internal name"); err != nil {
				return def, err
			}
		}
		if driver := text(object(value)["driver"]); driver != "" && driver != "bridge" {
			return def, fmt.Errorf("only bridge networks supported")
		}
	}
	for name, value := range object(raw["volumes"]) {
		if !namePattern.MatchString(name) {
			return def, fmt.Errorf("invalid volume key")
		}
		if value != nil {
			if err := allowed(object(value), "name"); err != nil {
				return def, err
			}
		}
	}
	delete(raw, "version")
	delete(raw, "name")
	data, err = json.Marshal(raw)
	if err != nil {
		return def, fmt.Errorf("invalid Compose values")
	}
	project, err := loader.LoadWithContext(ctx, types.ConfigDetails{WorkingDir: bundle.Directory, ConfigFiles: []types.ConfigFile{{Filename: file, Content: data}}, Environment: types.Mapping(bundle.Variables)}, func(o *loader.Options) {
		o.SetProjectName("attackbox-template", true)
		o.ResolvePaths = true
		o.SkipInclude = true
		o.SkipExtends = true
	})
	if err != nil {
		return def, fmt.Errorf("Compose validation failed; check fields, dependencies and required variables")
	}
	data, err = project.MarshalJSON()
	if err != nil {
		return def, fmt.Errorf("cannot normalize Compose")
	}
	if err = json.Unmarshal(data, &def.Document); err != nil {
		return def, err
	}
	def.Files = map[string]string{}
	def.OneShot = map[string]bool{}
	def.EntryService = bundle.EntryService
	def.EntryNetwork = bundle.EntryNetwork
	def.EntryPort = bundle.EntryPort
	def.PrefixBits = bundle.PrefixBits
	if def.PrefixBits == 0 {
		def.PrefixBits = 28
	}
	if def.PrefixBits < 24 || def.PrefixBits > 28 {
		return def, fmt.Errorf("network size must be /24 through /28")
	}
	docServices := object(def.Document["services"])
	names := keys(docServices)
	if def.EntryService == "" {
		def.EntryService = names[0]
		if _, ok := docServices["web"]; ok {
			def.EntryService = "web"
		}
	}
	entry := object(docServices[def.EntryService])
	if entry == nil || def.EntryPort < 0 || def.EntryPort > 65535 {
		return def, fmt.Errorf("invalid entry service or port")
	}
	def.Networks = keys(object(def.Document["networks"]))
	if len(def.Networks) == 0 || len(def.Networks) > 8 {
		return def, fmt.Errorf("one to eight project networks required")
	}
	if def.EntryNetwork == "" {
		entryNetworks := keys(object(entry["networks"]))
		if len(entryNetworks) == 0 {
			return def, fmt.Errorf("entry service needs network")
		}
		def.EntryNetwork = entryNetworks[0]
	}
	if _, ok := object(entry["networks"])[def.EntryNetwork]; !ok {
		return def, fmt.Errorf("entry service is not on selected network")
	}
	volumes := object(def.Document["volumes"])
	if volumes == nil {
		volumes = map[string]any{}
		def.Document["volumes"] = volumes
	}
	for _, name := range names {
		s := object(docServices[name])
		for key, value := range object(s["environment"]) {
			if value == nil {
				delete(object(s["environment"]), key)
			}
		}
		if s["container_name"] != nil {
			delete(s, "container_name")
			def.Report = append(def.Report, name+": container_name replaced by project identity")
		}
		if text(s["restart"]) != "" && text(s["restart"]) != "no" {
			def.Report = append(def.Report, name+": restart policy set to no")
		}
		s["restart"] = "no"
		s["pull_policy"] = "never"
		for _, netValue := range object(s["networks"]) {
			if cfg := object(netValue); cfg != nil {
				if err := allowed(cfg, "aliases"); err != nil {
					return def, err
				}
			}
		}
		for dep, config := range object(s["depends_on"]) {
			if text(object(config)["condition"]) == "service_completed_successfully" {
				def.OneShot[dep] = true
			}
		}
		mem := budget.MemoryBytes / int64(len(names))
		cpu := float64(budget.NanoCPUs) / 1e9 / float64(len(names))
		pids := budget.PIDs / int64(len(names))
		if mem < 16<<20 || pids < 8 {
			return def, fmt.Errorf("budget too small for service count")
		}
		for k, limit := range map[string]float64{"mem_limit": float64(mem), "cpus": cpu, "pids_limit": float64(pids)} {
			if value, ok := s[k].(float64); ok && value > 0 && value < limit {
				limit = value
			}
			s[k] = limit
		}
		s["memswap_limit"] = s["mem_limit"]
		s["logging"] = map[string]any{"driver": "json-file", "options": map[string]any{"max-size": "10m", "max-file": "2"}}
		// A finite stop grace prevents a project from monopolizing cleanup.
		s["stop_grace_period"] = "5s"
		mounts, _ := s["volumes"].([]any)
		for i, m := range mounts {
			mount := object(m)
			switch text(mount["type"]) {
			case "bind":
				if mount["read_only"] != true {
					return def, fmt.Errorf("package binds must be read-only")
				}
				if opts := object(mount["bind"]); opts != nil {
					if err := allowed(opts, "create_host_path"); err != nil {
						return def, err
					}
				}
				src, err := safePath(bundle.Directory, text(mount["source"]))
				if err != nil {
					return def, err
				}
				info, err := os.Stat(src)
				if err != nil {
					return def, err
				}
				if info.IsDir() {
					return def, fmt.Errorf("this round supports read-only package files; directory mounts need explicit packaging")
				}
				b, err := readBounded(src, 4<<20)
				if err != nil {
					return def, err
				}
				rel, _ := filepath.Rel(bundle.Directory, src)
				rel = filepath.ToSlash(rel)
				def.Files[rel] = digest(b)
				mount["source"] = "files/" + rel
				mount["bind"] = map[string]any{"create_host_path": false}
			case "volume":
				if mount["source"] == nil || mount["source"] == "" {
					key := fmt.Sprintf("anon_%s_%d", name, i)
					if _, ok := volumes[key]; ok {
						return def, fmt.Errorf("generated anonymous volume name collision")
					}
					mount["source"] = key
					volumes[key] = map[string]any{}
				}
				if opts := object(mount["volume"]); opts != nil {
					if err := allowed(opts, "nocopy"); err != nil {
						return def, err
					}
				}
			default:
				return def, fmt.Errorf("unsupported mount type")
			}
		}
	}
	for key := range volumes {
		volumes[key] = map[string]any{}
		def.Report = append(def.Report, key+": volume isolated per project")
	}
	if def.OneShot[def.EntryService] {
		return def, fmt.Errorf("entry service must be long-running")
	}
	for key := range object(def.Document["networks"]) {
		def.Document["networks"].(map[string]any)[key] = map[string]any{"internal": true}
		def.Report = append(def.Report, key+": internal network managed by platform")
	}
	def.Report = append(def.Report, "resource limits distributed within whole-project budget")
	sort.Strings(def.Report)
	delete(def.Document, "name")
	if len(def.Files) > 128 {
		return def, fmt.Errorf("too many package files")
	}
	return def, nil
}

func cloneDocument(doc map[string]any) map[string]any {
	b, _ := json.Marshal(doc)
	var out map[string]any
	json.Unmarshal(b, &out)
	return out
}

// The second CLI parse must preserve literal dollars resolved by compose-go,
// never interpolate the Worker's process environment into the frozen document.
func escapeDollars(v any) any {
	switch x := v.(type) {
	case string:
		return strings.ReplaceAll(x, "$", "$$")
	case []any:
		for i := range x {
			x[i] = escapeDollars(x[i])
		}
	case map[string]any:
		for k, value := range x {
			x[k] = escapeDollars(value)
		}
	}
	return v
}
