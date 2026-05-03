package engine

import (
	"github.com/ebad-arshad/kubepurge/pkg/types"
)

// DiffReceipt compares a new list of resources against an existing receipt.
// It returns a list of resources that are NEW (in manifest but not in receipt).
func DiffReceipt(oldReceipt types.Receipt, newResources []types.Resource) []types.Resource {
	var added []types.Resource

	// Create a map for quick lookup of existing resources
	existingMap := make(map[string]bool)
	for _, res := range oldReceipt.Resources {
		// Unique key: Kind:Name:Namespace
		key := res.Kind + ":" + res.Name + ":" + res.Namespace
		existingMap[key] = true
	}

	for _, res := range newResources {
		key := res.Kind + ":" + res.Name + ":" + res.Namespace
		if !existingMap[key] {
			added = append(added, res)
		}
	}

	return added
}