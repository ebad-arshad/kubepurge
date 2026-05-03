package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CleanupArchives keeps 'keep' versions of EACH unique receipt ID in the archive folder
func CleanupArchives(keep int) error {
	files, err := os.ReadDir(archiveDir)
	if err != nil {
		return err
	}

	// 1. Group files by their base ID
	// Key: "nginx-app", Value: List of historical file info
	buckets := make(map[string][]os.FileInfo)

	for _, file := range files {
		// Only process .json files and skip directories
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			info, err := file.Info()
			if err != nil {
				continue
			}

			// Format is [ID]-[Timestamp].json
			fileName := strings.TrimSuffix(file.Name(), ".json")
			parts := strings.Split(fileName, "-")

			if len(parts) < 2 {
				continue // Skip files that don't match our naming convention
			}

			// The last part is the timestamp, everything before is the original ID
			baseID := strings.Join(parts[:len(parts)-1], "-")
			buckets[baseID] = append(buckets[baseID], info)
		}
	}

	// 2. Process each bucket to enforce retention
	for baseID, archiveList := range buckets {
		if len(archiveList) <= keep {
			continue // Within limits
		}

		// Sort by modification time (Newest first)
		sort.Slice(archiveList, func(i, j int) bool {
			return archiveList[i].ModTime().After(archiveList[j].ModTime())
		})

		// 3. Delete the oldest versions
		toDelete := archiveList[keep:]
		fmt.Printf("🧹 Resource '%s' has %d archives. Keeping %d most recent...\n", baseID, len(archiveList), keep)

		for _, file := range toDelete {
			err := os.Remove(filepath.Join(archiveDir, file.Name()))
			if err != nil {
				fmt.Printf("⚠️  Failed to delete %s: %v\n", file.Name(), err)
			} else {
				fmt.Printf("🗑️  Removed old archive: %s\n", file.Name())
			}
		}
	}

	return nil
}