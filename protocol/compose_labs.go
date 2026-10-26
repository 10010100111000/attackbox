package protocol

import "time"

const (
	MsgComposePrepare       = "compose_prepare"
	MsgComposePrepareStatus = "compose_prepare_status"
	MsgComposeStart         = "compose_start"
	MsgComposeInspect       = "compose_inspect"
	MsgComposeStop          = "compose_stop"
)

type ComposeSettings struct {
	File         string `json:"file"`
	EntryService string `json:"entryService"`
	EntryNetwork string `json:"entryNetwork"`
	EntryPort    int    `json:"entryPort"`
	PrefixBits   int    `json:"prefixBits"`
}
type ComposeRequest struct {
	BootID        string            `json:"boot_id"`
	TargetID      string            `json:"target_id"`
	OwnerID       string            `json:"owner_id"`
	PreparationID string            `json:"preparation_id"`
	DeploymentID  string            `json:"deployment_id"`
	BundleID      string            `json:"bundle_id"`
	BundleHash    string            `json:"bundle_hash"`
	Settings      ComposeSettings   `json:"settings"`
	Variables     map[string]string `json:"variables"`
	ExpiresAt     time.Time         `json:"expires_at"`
}
type ComposeCapability struct {
	Available       bool   `json:"available"`
	NetworkVerified bool   `json:"network_verified"`
	ProtocolVersion int    `json:"protocol_version"`
	Error           string `json:"error,omitempty"`
}
type ComposeService struct {
	Name   string            `json:"name"`
	State  string            `json:"state"`
	Health string            `json:"health"`
	IPs    map[string]string `json:"ips"`
}
type ComposeInstance struct {
	AttackerAddress string           `json:"attacker_address,omitempty"`
	Interface       string           `json:"interface,omitempty"`
	TargetID        string           `json:"target_id"`
	OwnerID         string           `json:"owner_id"`
	State           string           `json:"state"`
	Ready           bool             `json:"ready"`
	Readiness       string           `json:"readiness"`
	EntryAddress    string           `json:"entry_address,omitempty"`
	Attachment      string           `json:"attachment"`
	Services        []ComposeService `json:"services"`
	ExpiresAt       time.Time        `json:"expires_at"`
}
type ComposeServiceDefinition struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}
type ComposeDefinition struct {
	Services     []ComposeServiceDefinition `json:"services"`
	Networks     []string                   `json:"networks"`
	EntryService string                     `json:"entry_service"`
	EntryNetwork string                     `json:"entry_network"`
	EntryPort    int                        `json:"entry_port"`
	Report       []string                   `json:"report"`
}
type ComposeResponse struct {
	Logs         []string           `json:"logs,omitempty"`
	Success      bool               `json:"success"`
	BootID       string             `json:"boot_id"`
	TargetID     string             `json:"target_id"`
	OwnerID      string             `json:"owner_id"`
	JobState     string             `json:"job_state,omitempty"`
	DeploymentID string             `json:"deployment_id,omitempty"`
	Summary      *ComposeDefinition `json:"summary,omitempty"`
	Instance     *ComposeInstance   `json:"instance,omitempty"`
	Error        string             `json:"error,omitempty"`
}
