package protocol

const (
	MsgTypeTargetImagePrepare = "target_image_prepare"
	MsgTypeTargetImageStatus  = "target_image_status"
	MsgTypeTargetStart        = "target_start"
	MsgTypeTargetInspect      = "target_inspect"
	MsgTypeTargetStop         = "target_stop"
)

type TargetPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

// TargetID identifies an immutable instance. A stopped instance cannot be
// restarted; the controller must allocate a new ID for a new lifecycle.
type TargetRequest struct {
	BootID   string       `json:"boot_id"`
	TargetID string       `json:"target_id"`
	OwnerID  string       `json:"owner_id"`
	Image    string       `json:"image,omitempty"`
	Ports    []TargetPort `json:"ports,omitempty"`
}

type TargetSummary struct {
	TargetID    string `json:"target_id"`
	OwnerID     string `json:"owner_id"`
	ContainerID string `json:"container_id,omitempty"`
	State       string `json:"state"`
	IP          string `json:"ip,omitempty"`
	Ready       bool   `json:"ready"`
	Readiness   string `json:"readiness"`
}

type TargetResponse struct {
	ImageState string         `json:"image_state,omitempty"`
	BootID     string         `json:"boot_id"`
	TargetID   string         `json:"target_id"`
	Success    bool           `json:"success"`
	Error      string         `json:"error,omitempty"`
	Instance   *TargetSummary `json:"instance,omitempty"`
}

// Available=false means inventory is unknown, never that all containers vanished.
type TargetInventory struct {
	Available bool            `json:"available"`
	Error     string          `json:"error,omitempty"`
	Instances []TargetSummary `json:"instances"`
}
