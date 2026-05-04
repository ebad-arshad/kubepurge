package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CleanupArchives keeps 'keep' versions of EACH unique receipt ID in the archive folder
func CleanupArchives(keep int, targetID string) error {
    files, err := os.ReadDir(archiveDir)
    if err != nil {
        return err
    }

    buckets := make(map[string][]os.FileInfo)

    for _, entry := range files {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
            continue
        }

        info, _ := entry.Info()
        name := strings.TrimSuffix(entry.Name(), ".json")
        
        // 1. EXTRACT the baseID first
        var baseID string
        if strings.Contains(name, "-") {
            parts := strings.Split(name, "-")
            baseID = strings.Join(parts[:len(parts)-1], "-")
        } else if strings.Contains(name, "_") {
            parts := strings.Split(name, "_")
            baseID = strings.Join(parts[:len(parts)-1], "_")
        } else {
            baseID = name 
        }

        // 2. NOW check if it matches the targetID (if one was provided)
        if targetID != "" && baseID != targetID {
            continue
        }

        buckets[baseID] = append(buckets[baseID], info)
    }

    if len(buckets) == 0 {
        fmt.Println("ℹ️  No matching archive files found to clean.")
        return nil
    }

    for id, list := range buckets {
        if len(list) <= keep {
            fmt.Printf("✅ '%s' is already clean (Files: %d, Keep: %d)\n", id, len(list), keep)
            continue
        }

        // Sort: Newest First
        sort.Slice(list, func(i, j int) bool {
            return list[i].ModTime().After(list[j].ModTime())
        })

        toDelete := list[keep:]
        fmt.Printf("🧹 Pruning '%s': Keeping %d, deleting %d...\n", id, keep, len(toDelete))

        for _, f := range toDelete {
            err := os.Remove(filepath.Join(archiveDir, f.Name()))
            if err != nil {
                fmt.Printf("⚠️  Failed to delete %s: %v\n", f.Name(), err)
            } else {
                fmt.Printf("🗑️  Deleted: %s\n", f.Name())
            }
        }
    }
    return nil
}