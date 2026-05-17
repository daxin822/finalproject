package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
)

//go:embed data/*.json
var dataFS embed.FS

type ProvisionMode string

const (
	ProvisionAVITemplate ProvisionMode = "avi_template"
	ProvisionHardSplit   ProvisionMode = "hard_split"
)

type VirtualizationMode string

const (
	VirtSoftware VirtualizationMode = "software"
	VirtHardware VirtualizationMode = "hardware"
	VirtNone     VirtualizationMode = "none"
)

type SliceFlavor struct {
	ID                  string             `json:"id"`
	DisplayName         string             `json:"display_name"`
	HardwareFamily      string             `json:"hardware_family"`
	AVITemplateID       string             `json:"avi_template_id,omitempty"`
	ProvisionMode       ProvisionMode      `json:"provision_mode"`
	VirtualizationMode  VirtualizationMode `json:"virtualization_mode"`
	AICores             int                `json:"ai_cores"`
	MemoryMiB           int                `json:"memory_mib"`
	AICPUs              int                `json:"ai_cpus"`
	K8sExtendedResource string             `json:"k8s_extended_resource"`
	PodLimitsExample    map[string]string  `json:"pod_limits_example"`
	Notes               string             `json:"notes,omitempty"`
}

type Pool struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	NodeSelector       map[string]string `json:"node_selector"`
	SupportedFlavorIDs []string          `json:"supported_flavor_ids"`
}

type Root struct {
	Version int           `json:"version"`
	Flavors []SliceFlavor `json:"flavors"`
	Pools   []Pool        `json:"pools"`
}

func LoadEmbedded() (*Root, error) {
	b, err := fs.ReadFile(dataFS, "data/avi_slice_catalog.json")
	if err != nil {
		return nil, err
	}
	var root Root
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, err
	}
	if root.Version == 0 {
		return nil, fmt.Errorf("catalog: missing version")
	}
	return &root, nil
}

func (r *Root) FlavorByID(id string) (SliceFlavor, bool) {
	for _, f := range r.Flavors {
		if f.ID == id {
			return f, true
		}
	}
	return SliceFlavor{}, false
}

func (r *Root) PoolByID(id string) (Pool, bool) {
	for _, p := range r.Pools {
		if p.ID == id {
			return p, true
		}
	}
	return Pool{}, false
}
