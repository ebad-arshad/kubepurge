package types

import "time"

// Resource represents a single Kubernetes object tracked by KubePurge
type Resource struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"api_version"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"` // Empty for cluster-scoped resources
}

// Receipt is the JSON structure saved to disk after an installation
type Receipt struct {
	ID          string     `json:"id"`           // Unique name for this installation (e.g., "calico-v3")
	AppliedAt   time.Time  `json:"applied_at"`
	Resources   []Resource `json:"resources"`
}

// ReceiptSummary gives a quick overview of a saved receipt
type ReceiptSummary struct {
	ID        string
	ResourceCount int
	AppliedAt time.Time
}