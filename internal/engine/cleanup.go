package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CleanupArchives keeps 'keep' versions of EACH unique receipt ID
func CleanupArchives(keep int) error {
	files, err := os.ReadDir(receiptDir)
	if err != nil {
		return err
	}

	// 1. Group files by their base ID
	// Key: "calico-v5", Value: List of full filenames
	buckets := make(map[string][]os.FileInfo)

	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "archived-") {
			info, _ := file.Info()
			
			// Extract the base ID (e.g., "archived-calico-v5-2026.json" -> "calico-v5")
			name := strings.TrimPrefix(file.Name(), "archived-")
			name = strings.TrimSuffix(name, ".json")
			
			// We split by the timestamp dash we added in ArchiveReceipt
			parts := strings.Split(name, "-")
			baseID := parts[0] 
			if len(parts) > 1 && !strings.Contains(parts[len(parts)-1], "202") {
				// This handles IDs that might have dashes in them
				baseID = strings.Join(parts[:len(parts)-1], "-")
			}

			buckets[baseID] = append(buckets[baseID], info)
		}
	}

	// 2. Process each bucket
	for baseID, archiveList := range buckets {
		if len(archiveList) <= keep {
			continue // Nothing to do for this resource
		}

		// Sort by modification time (Newest first)
		sort.Slice(archiveList, func(i, j int) bool {
			return archiveList[i].ModTime().After(archiveList[j].ModTime())
		})

		// 3. Delete the extras
		toDelete := archiveList[keep:]
		fmt.Printf("🧹 Resource '%s' has %d archives. Keeping %d...\n", baseID, len(archiveList), keep)
		
		for _, file := range toDelete {
			err := os.Remove(filepath.Join(receiptDir, file.Name()))
			if err != nil {
				fmt.Printf("⚠️  Failed to delete %s: %v\n", file.Name(), err)
			}
		}
	}

	return nil
}