// Package labcompose manages prepared, private Compose lab projects behind the
// Worker control protocol, independently of the legacy single-container engine.
package labcompose

import (
	"context"
	"time"

	"attackbox/node/labnetwork"
)

// ComposeVersion is the tested baseline, not an exact installation requirement.
const ComposeVersion = "2.40.3"
const labelPrefix = "io.attackbox.compose."

type Budget struct {
	MemoryBytes int64 `json:"memory_bytes"`
	NanoCPUs    int64 `json:"nano_cpus"`
	PIDs        int64 `json:"pids"`
	MaxServices int   `json:"max_services"`
}
type Bundle struct {
	Directory, File            string
	Variables                  map[string]string
	EntryService, EntryNetwork string
	EntryPort                  int
	PrefixBits                 int
}
type Definition struct {
	Builds       map[string]BuildSpec `json:"builds,omitempty"`
	Directories  []string             `json:"directories,omitempty"`
	Document     map[string]any       `json:"document"`
	Files        map[string]string    `json:"files"` // Relative package file -> SHA256.
	Networks     []string             `json:"networks"`
	EntryService string               `json:"entry_service"`
	EntryNetwork string               `json:"entry_network"`
	EntryPort    int                  `json:"entry_port"`
	PrefixBits   int                  `json:"prefix_bits"`
	OneShot      map[string]bool      `json:"one_shot"`
	Report       []string             `json:"report"`
}
type Deployment struct {
	ID         string     `json:"id"`
	Definition Definition `json:"definition"`
}
type Request struct {
	ID           string    `json:"id"`
	OwnerID      string    `json:"owner_id"`
	DeploymentID string    `json:"deployment_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}
type Record struct {
	NodeID      string                        `json:"node_id"`
	StartIssued bool                          `json:"start_issued"`
	VolumeNames []string                      `json:"volume_names"`
	Request     Request                       `json:"request"`
	Project     string                        `json:"project"`
	Desired     bool                          `json:"desired"`
	Cleaned     bool                          `json:"cleaned"`
	Networks    map[string]labnetwork.Network `json:"networks"`
}
type Service struct {
	Name   string            `json:"name"`
	State  string            `json:"state"`
	Health string            `json:"health"`
	IPs    map[string]string `json:"ips"`
}
type Status struct {
	ID              string    `json:"id"`
	State           string    `json:"state"`
	Ready           bool      `json:"ready"`
	Readiness       string    `json:"readiness"`
	Services        []Service `json:"services"`
	EntryAddress    string    `json:"entry_address,omitempty"`
	ExpiresAt       time.Time `json:"expires_at"`
	Attachment      string    `json:"attachment"`
	AttackerAddress string    `json:"attacker_address,omitempty"`
	Interface       string    `json:"interface,omitempty"`
}

// Only this interface abstracts Docker execution. Implementations must refuse
// project-name collisions before any mutation and confirm resource absence.
type Engine interface {
	Check(context.Context) error
	PrepareImage(context.Context, string) (string, error)
	ImagePresent(context.Context, string) error
	Up(context.Context, Record, string) error
	Inspect(context.Context, Record) ([]Service, error)
	Down(context.Context, Record, string) error
}
type Networks interface {
	CreateNetwork(context.Context, labnetwork.NetworkSpec) (labnetwork.Network, error)
	ReleaseNetwork(context.Context, string) error
	Attach(context.Context, string, labnetwork.Sandbox) (labnetwork.Attachment, error)
	Detach(context.Context, string) error
	Snapshot() ([]labnetwork.Network, []labnetwork.Attachment)
}
