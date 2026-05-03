package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ebad-arshad/kubepurge/pkg/types"
)

var (
	baseDir    string
	activeDir  string
	archiveDir string
	deletedDir string
)

// init() runs automatically when the package is imported
func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp" // Fallback if home dir cannot be found
	}

	baseDir = filepath.Join(home, ".kubepurge")
	activeDir = filepath.Join(baseDir, "active")
	archiveDir = filepath.Join(baseDir, "archive")
	deletedDir = filepath.Join(baseDir, "deleted")

	// Ensure all directories exist
	os.MkdirAll(activeDir, 0755)
	os.MkdirAll(archiveDir, 0755)
	os.MkdirAll(deletedDir, 0755)
}

// SaveReceipt writes the resource list to a JSON file in the active directory.
func SaveReceipt(receipt types.Receipt) error {
	filePath := filepath.Join(activeDir, receipt.ID+".json")

	// 1. Check if ACTIVE file exists
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("ID '%s' is currently active. Use --replaces if you want to update it", receipt.ID)
	}

	// 2. Check if an ARCHIVE of this ID exists
	// We check the archive folder for files starting with "ID-" (e.g., my-app-16839393.json)
	files, _ := os.ReadDir(archiveDir)
	for _, file := range files {
		if strings.HasPrefix(file.Name(), receipt.ID+"-") && strings.HasSuffix(file.Name(), ".json") {
			return fmt.Errorf("ID '%s' already exists in your archives. Please use a new version name (e.g., %s-v2) to avoid confusion", receipt.ID, receipt.ID)
		}
	}

	// 3. Proceed with saving if clear
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal receipt: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadReceipt reads a saved receipt from the active directory by its ID.
func LoadReceipt(id string) (*types.Receipt, error) {
	filePath := filepath.Join(activeDir, id+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("receipt '%s' not found in active tracking", id)
	}

	var receipt types.Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, fmt.Errorf("failed to parse receipt: %w", err)
	}

	return &receipt, nil
}

// ArchiveReceipt moves a receipt from the active directory to the archive directory.
func ArchiveReceipt(id string) error {
	activePath := filepath.Join(activeDir, id+".json")

	// Check if the old receipt actually exists in the active folder
	if _, err := os.Stat(activePath); os.IsNotExist(err) {
		return fmt.Errorf("receipt '%s' not found in active directory", id)
	}

	// Create a clean archive name using a Unix timestamp
	timestamp := time.Now().Unix()
	archiveName := fmt.Sprintf("%s-%d.json", id, timestamp)
	archivePath := filepath.Join(archiveDir, archiveName)

	// Move the file
	return os.Rename(activePath, archivePath)
}

// ListActiveReceipts returns a summary of all deployments currently tracked in the active directory.
func ListActiveReceipts() ([]types.ReceiptSummary, error) {
	files, err := os.ReadDir(activeDir)
	if err != nil {
		return nil, err
	}

	var summaries []types.ReceiptSummary
	for _, file := range files {
		// Only process .json files. No need to check for "archived-" prefixes anymore!
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			receipt, err := LoadReceipt(strings.TrimSuffix(file.Name(), ".json"))
			if err != nil {
				continue
			}
			summaries = append(summaries, types.ReceiptSummary{
				ID:            receipt.ID,
				ResourceCount: len(receipt.Resources),
				AppliedAt:     receipt.AppliedAt,
			})
		}
	}
	return summaries, nil
}

// MoveToDeleted moves a receipt from active to deleted after a purge.
func MoveToDeleted(id string) error {
	activePath := filepath.Join(activeDir, id+".json")

	if _, err := os.Stat(activePath); os.IsNotExist(err) {
		return fmt.Errorf("receipt '%s' not found in active directory", id)
	}

	timestamp := time.Now().Unix()
	deletedName := fmt.Sprintf("%s-purged-%d.json", id, timestamp)
	deletedPath := filepath.Join(deletedDir, deletedName)

	return os.Rename(activePath, deletedPath)
}