// Package labnetwork implements the opt-in, local network attachment layer for
// Compose labs. It is deliberately independent of the legacy target runtime.
package labnetwork

import (
	"context"
	"net/netip"
)

const (
	labelNode  = "io.attackbox.lab.node"
	labelKey   = "io.attackbox.lab.key"
	labelOwner = "io.attackbox.lab.owner"
)

type Config struct {
	NodeID       string
	StateDir     string
	DockerSocket string
	Pool         netip.Prefix
	// Excluded includes sandbox, VPN and other ranges reserved by the operator.
	Excluded []netip.Prefix
}

type NetworkSpec struct {
	TargetID   string `json:"target_id"`
	OwnerID    string `json:"owner_id"`
	Key        string `json:"key"` // Compose logical network key, not a host name.
	PrefixBits int    `json:"prefix_bits"`
}

type Network struct {
	Spec      NetworkSpec  `json:"spec"`
	ID        string       `json:"id"` // Stable platform identity, not Docker's ID.
	Name      string       `json:"name"`
	Bridge    string       `json:"bridge"`
	Subnet    netip.Prefix `json:"subnet"`
	Gateway   netip.Addr   `json:"gateway"`
	SandboxIP netip.Addr   `json:"sandbox_ip"`
	DockerID  string       `json:"docker_id,omitempty"`
	Desired   bool         `json:"desired"`
	Cleaned   bool         `json:"cleaned"`
}

type NamespaceIdentity struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

// Namespace must be captured when the sandbox is created, not first observed
// from a user-supplied PID at attachment time. The caller serializes sandbox
// teardown with attachment and supplies the sandbox's current immutable ID.
type Sandbox struct {
	ID        string            `json:"id"`
	OwnerID   string            `json:"owner_id"`
	PID       int               `json:"pid"`
	Namespace NamespaceIdentity `json:"namespace"`
}

type Attachment struct {
	ID        string       `json:"id"`
	NetworkID string       `json:"network_id"`
	Sandbox   Sandbox      `json:"sandbox"`
	HostName  string       `json:"host_name"`
	GuestName string       `json:"guest_name"`
	PeerName  string       `json:"peer_name"`
	Alias     string       `json:"alias"`
	Address   netip.Prefix `json:"address"`
	Desired   bool         `json:"desired"`
	Cleaned   bool         `json:"cleaned"`
}

// ReconcileReport reports drift; it never treats missing resources as proof
// that all cleanup obligations have completed, or attaches to a new PID.
type ReconcileReport struct {
	Pending  []string
	Detached []string
	Orphans  []string
}

type dockerBackend interface {
	Networks(context.Context) ([]dockerNetwork, error)
	Ensure(context.Context, Network, string) (string, error)
	Verify(context.Context, Network, string) error
	Remove(context.Context, Network, string) error
	Close()
}

type linkBackend interface {
	Occupied(context.Context) ([]netip.Prefix, error)
	Ensure(context.Context, Network, Attachment, *PinnedNamespace) error
	Remove(context.Context, Attachment) error
	Check(context.Context, Network, Attachment, *PinnedNamespace) (bool, error)
	CheckBridgeEmpty(context.Context, Network) error
	Orphans(context.Context, string, map[string]bool) ([]string, error)
	Close()
}
