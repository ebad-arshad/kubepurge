package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"strings"
	"text/tabwriter"

	"github.com/ebad-arshad/kubepurge/internal/engine"
	"github.com/ebad-arshad/kubepurge/pkg/types"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"runtime/debug"
)

var (
	manifestPath string
	receiptID    string
	namespace    string
	replacesID   string
	keepCount 	 int
	forceCleanup bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "kubepurge",
		Short: "A tool to track and deep-clean K8s resources",
	}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of KubePurge",
		Run: func(cmd *cobra.Command, args []string) {
			if info, ok := debug.ReadBuildInfo(); ok {
				fmt.Printf("KubePurge %s\n", info.Main.Version)
				return
			}
			fmt.Println("KubePurge version unknown")
		},
	}

	var statusCmd = &cobra.Command{
		Use:   "status [ID]",
		Short: "Check the live status of resources in a receipt",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			isArchive := strings.HasPrefix(id, "archived-")

			receipt, err := engine.LoadReceipt(id)
			if err != nil {
				log.Fatalf("❌ Error: %v", err)
			}

			// --- NEW: Visual Labeling ---
			if isArchive {
				fmt.Println("⚠️  [ARCHIVE RECORD]")
				fmt.Printf("📡 Checking live status for historical ID: %s\n", id)
			} else {
				fmt.Printf("📡 Checking live cluster status for active ID: %s\n", id)
			}
			// ----------------------------

			dynClient, err := getDynamicClient()
			if err != nil {
				log.Fatalf("❌ K8s Connection Error: %v", err)
			}

			results, err := engine.CheckStatus(dynClient, receipt.Resources)
			if err != nil {
				log.Fatalf("❌ Status check failed: %v", err)
			}

			for _, line := range results {
				// If it's an archive, we could even prefix the lines to be extra clear
				if isArchive {
					fmt.Printf("[ARCHIVE] %s\n", line)
				} else {
					fmt.Println(line)
				}
			}
		},
	}

	var diffCmd = &cobra.Command{
	Use:   "diff [ID] -f [MANIFEST]",
	Short: "Compare a manifest file against a saved receipt",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			if manifestPath == "" {
				log.Fatal("❌ Error: --file (-f) is required to perform a diff")
			}

			// 1. Load the old receipt
			oldReceipt, err := engine.LoadReceipt(id)
			if err != nil {
				log.Fatalf("❌ Error loading receipt: %v", err)
			}

			// 2. Read and parse the new manifest
			data, err := readManifest(manifestPath)
			if err != nil {
				log.Fatalf("❌ Error reading manifest: %v", err)
			}

			newResources, err := engine.ExtractResources(data, namespace)
			if err != nil {
				log.Fatalf("❌ Error parsing manifest: %v", err)
			}

			// 3. Find the differences
			added := engine.DiffReceipt(*oldReceipt, newResources)

			if len(added) == 0 {
				fmt.Printf("✅ No changes! Your local manifest matches the tracking record '%s'.\n", id)
				fmt.Println("💡 Tip: Use 'status' to check if these resources are actually running in the cluster.")
				return
			}

			fmt.Printf("⚠️  Found %d new resources in the manifest not tracked by '%s':\n\n", len(added), id)
			
			w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
			fmt.Fprintln(w, "KIND\tNAME\tNAMESPACE")
			for _, res := range added {
				ns := res.Namespace
				if ns == "" { ns = "(cluster-scoped)" }
				fmt.Fprintf(w, "%s\t%s\t%s\n", res.Kind, res.Name, ns)
			}
			w.Flush()

			fmt.Printf("\n👉 Recommendation: Run 'apply -i %s -f %s --replaces %s' to update your tracking.\n", id+"-new", manifestPath, id)
		},
	}

	var inspectCmd = &cobra.Command{
	Use:   "inspect [ID]",
	Short: "View resources contained within a receipt",
	Args:  cobra.ExactArgs(1), // This forces the user to provide exactly one ID
	Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			
			receipt, err := engine.LoadReceipt(id)
			if err != nil {
				log.Fatalf("❌ Error: %v", err)
			}

			fmt.Printf("📄 Receipt ID: %s\n", receipt.ID)
			fmt.Printf("📅 Created:    %s\n", receipt.AppliedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("📦 Resources:  %d\n\n", len(receipt.Resources))

			w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
			fmt.Fprintln(w, "KIND\tNAME\tNAMESPACE")

			for _, res := range receipt.Resources {
				ns := res.Namespace
				if ns == "" {
					ns = "(cluster-scoped)"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", res.Kind, res.Name, ns)
			}
			w.Flush()
		},
	}

	var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active receipts",
	Run: func(cmd *cobra.Command, args []string) {
			summaries, err := engine.ListActiveReceipts()
			if err != nil {
				log.Fatalf("❌ Failed to list receipts: %v", err)
			}

			if len(summaries) == 0 {
				fmt.Println("Empty. No active receipts found.")
				return
			}

			// Initialize tabwriter for clean columns
			w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tRESOURCES\tAGE")

			for _, s := range summaries {
				// Calculate human-readable age
				age := time.Since(s.AppliedAt).Round(time.Second).String()
				fmt.Fprintf(w, "%s\t%d\t%s\n", s.ID, s.ResourceCount, age)
			}
			w.Flush()
		},
	}

	var cleanupCmd = &cobra.Command{
		Use:   "cleanup",
		Short: "Remove old archived receipts per resource",
		Run: func(cmd *cobra.Command, args []string) {
			// 1. Check for confirmation
			if !forceCleanup {
				fmt.Printf("⚠️  This will delete all but the %d most recent archives per resource. Continue? (y/N): ", keepCount)
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" {
					fmt.Println("❌ Cleanup cancelled.")
					return
				}
			}

			// 2. Run the actual logic
			err := engine.CleanupArchives(keepCount)
			if err != nil {
				log.Fatalf("❌ Cleanup failed: %v", err)
			}
			fmt.Println("✨ Cleanup finished!")
		},
	}

	// --- APPLY COMMAND ---
	var applyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Record a receipt and suggest manual installation",
		Run: func(cmd *cobra.Command, args []string) {
			if manifestPath == "" || receiptID == "" {
				log.Fatal("❌ Error: --file and --id are required")
			}

			// 1. Fetch and Parse
			data, err := readManifest(manifestPath)
			if err != nil {
				log.Fatalf("❌ Read failed: %v", err)
			}

			resources, err := engine.ExtractResources(data, namespace)
			if err != nil {
				log.Fatalf("❌ Parse failed: %v", err)
			}

			receipt := types.Receipt{
				ID:        receiptID,
				AppliedAt: time.Now(),
				Resources: resources,
			}

			// 2. SAVE the new receipt FIRST
			// This will trigger our "Existence Check" in SaveReceipt. 
			// If it fails, the program stops here and nothing is archived!
			if err := engine.SaveReceipt(receipt); err != nil {
				log.Fatalf("❌ Save failed: %v", err)
			}

			// 3. ARCHIVE the old one ONLY after the new one is safely saved
			if replacesID != "" {
				fmt.Printf("🔄 Success! Archiving old receipt '%s'...\n", replacesID)
				err := engine.ArchiveReceipt(replacesID)
				if err != nil {
					log.Printf("⚠️  Note: Could not archive %s: %v", replacesID, err)
				}
			}

			fmt.Printf("✅ Receipt '%s' created with %d resources.\n", receiptID, len(resources))
			fmt.Printf("👉 Next Step: kubectl apply -f %s\n", manifestPath)
		},
	}

	// --- PURGE COMMAND ---
	var purgeCmd = &cobra.Command{
		Use:   "purge",
		Short: "Deep clean resources using a receipt ID",
		Run: func(cmd *cobra.Command, args []string) {
			if receiptID == "" {
				log.Fatal("❌ Error: --id is required")
			}

			receipt, err := engine.LoadReceipt(receiptID)
			if err != nil {
				log.Fatalf("❌ Error: %v", err)
			}

			fmt.Printf("🔍 Found receipt '%s' (created %v)\n", receipt.ID, receipt.AppliedAt.Format("2006-01-02 15:04"))
			fmt.Printf("🚀 Starting Deep Purge for %d resources...\n", len(receipt.Resources))

			dynClient, err := getDynamicClient()
			if err != nil {
				log.Fatalf("❌ Could not connect to K8s: %v", err)
			}

			err = engine.PurgeResources(dynClient, receipt.Resources)
			if err != nil {
				log.Printf("⚠️  Purge finished with some warnings. Receipt kept in active status.")
			} else {
				// Archive upon successful purge
				err := engine.ArchiveReceipt(receiptID)
				if err != nil {
					log.Printf("⚠️  Could not archive receipt: %v", err)
				} else {
					fmt.Printf("✨ Purge complete! History archived.\n")
				}
			}
		},
	}

	// Define Flags for Apply
	applyCmd.Flags().StringVarP(&manifestPath, "file", "f", "", "YAML file or URL")
	applyCmd.Flags().StringVarP(&receiptID, "id", "i", "", "Unique ID for the receipt")
	applyCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Target namespace")
	applyCmd.Flags().StringVar(&replacesID, "replaces", "", "ID of an old receipt to archive")

	// Define Flags for Purge
	purgeCmd.Flags().StringVarP(&receiptID, "id", "i", "", "ID of the receipt to purge")

	// Define Flags for Cleanup
	cleanupCmd.Flags().IntVar(&keepCount, "keep", 5, "Number of archives to keep PER resource")
	cleanupCmd.Flags().BoolVarP(&forceCleanup, "force", "y", false, "Skip confirmation prompt")

	// Define Flags for Differentiate
	diffCmd.Flags().StringVarP(&manifestPath, "file", "f", "", "Manifest file or URL to compare")
	
	// Add commands to root
	rootCmd.AddCommand(applyCmd, purgeCmd, cleanupCmd, listCmd, inspectCmd, diffCmd, statusCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getDynamicClient() (dynamic.Interface, error) {
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(config)
}

func readManifest(path string) ([]byte, error) {
	if len(path) > 4 && path[:4] == "http" {
		resp, err := http.Get(path)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}
	return os.ReadFile(path)
}