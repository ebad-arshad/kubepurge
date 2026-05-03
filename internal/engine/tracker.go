package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"strings"
	"github.com/ebad-arshad/kubepurge/pkg/types"
)


func getReceiptDir() string {
    home, _ := os.UserHomeDir()
    path := filepath.Join(home, ".kubepurge", "receipts")
    
    // Ensure the directory exists
    os.MkdirAll(path, 0755)
    return path
}

var receiptDir = getReceiptDir()

// SaveReceipt writes the resource list to a JSON file, but prevents overwriting.
func SaveReceipt(receipt types.Receipt) error {
	if err := os.MkdirAll(receiptDir, 0755); err != nil {
		return fmt.Errorf("failed to create receipt directory: %w", err)
	}

	filePath := filepath.Join(receiptDir, receipt.ID+".json")

	// 1. Check if ACTIVE file exists
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("ID '%s' is currently active. Use --replaces if you want to update it", receipt.ID)
	}

	// 2. Check if an ARCHIVE of this ID exists
	// We check for any file starting with "archived-[ID]"
	files, _ := os.ReadDir(receiptDir)
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "archived-"+receipt.ID) {
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

// LoadReceipt reads a saved receipt from disk by its ID.
func LoadReceipt(id string) (*types.Receipt, error) {
	filePath := filepath.Join(receiptDir, id+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("receipt '%s' not found", id)
	}

	var receipt types.Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, fmt.Errorf("failed to parse receipt: %w", err)
	}

	return &receipt, nil
}

func ArchiveReceipt(id string) error {
	oldPath := filepath.Join(receiptDir, id+".json")
	
	// Check if the old receipt actually exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return fmt.Errorf("receipt %s not found", id)
	}

	newPath := filepath.Join(receiptDir, "archived-"+id+".json")
	
	// If an archive already exists with that name, add a timestamp to prevent overwrite
	if _, err := os.Stat(newPath); err == nil {
		timestamp := time.Now().Format("20060102-150405")
		newPath = filepath.Join(receiptDir, "archived-"+id+"-"+timestamp+".json")
	}

	return os.Rename(oldPath, newPath)
}

func ListActiveReceipts() ([]types.ReceiptSummary, error) {
	files, err := os.ReadDir(receiptDir)
	if err != nil {
		return nil, err
	}

	var summaries []types.ReceiptSummary
	for _, file := range files {
		// Only show .json files and ignore archived ones
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") && !strings.HasPrefix(file.Name(), "archived-") {
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