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
    "os/exec"
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
    forceCleanup bool
	keepCount 	 int
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
        Use:   "status",
        Short: "Check the live status of resources in a receipt",
        // No positional args allowed now, must use -i
        Args:  cobra.NoArgs, 
        Run: func(cmd *cobra.Command, args []string) {
            if receiptID == "" {
                log.Fatal("❌ Error: --id (-i) is required to check status")
            }

            // We load from active by default using the global receiptID
            receipt, err := engine.LoadReceipt(receiptID)
            if err != nil {
                log.Fatalf("❌ Error: %v", err)
            }

            fmt.Printf("📡 Checking live cluster status for active ID: %s\n", receiptID)

            dynClient, err := getDynamicClient()
            if err != nil {
                log.Fatalf("❌ K8s Connection Error: %v", err)
            }

            results, err := engine.CheckStatus(dynClient, receipt.Resources)
            if err != nil {
                log.Fatalf("❌ Status check failed: %v", err)
            }

            for _, line := range results {
                fmt.Println(line)
            }
        },
    }

    var diffCmd = &cobra.Command{
        Use:   "diff",
        Short: "Compare a manifest file against a saved receipt",
        Args:  cobra.NoArgs, // No positional arguments, strictly uses flags
        Run: func(cmd *cobra.Command, args []string) {
            if receiptID == "" || manifestPath == "" {
                log.Fatal("❌ Error: Both --id (-i) and --file (-f) are required to perform a diff")
            }

            // Load receipt using the global receiptID
            oldReceipt, err := engine.LoadReceipt(receiptID)
            if err != nil {
                log.Fatalf("❌ Error loading receipt: %v", err)
            }

            data, err := readManifest(manifestPath)
            if err != nil {
                log.Fatalf("❌ Error reading manifest: %v", err)
            }

            newResources, err := engine.ExtractResources(data, namespace)
            if err != nil {
                log.Fatalf("❌ Error parsing manifest: %v", err)
            }

            added := engine.DiffReceipt(*oldReceipt, newResources)

            if len(added) == 0 {
                fmt.Printf("✅ No changes! Your local manifest matches tracking record '%s'.\n", receiptID)
                return
            }

            fmt.Printf("⚠️  Found %d new resources in the manifest not tracked by '%s':\n\n", len(added), receiptID)
            
            w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
            fmt.Fprintln(w, "KIND\tNAME\tNAMESPACE")
            for _, res := range added {
                ns := res.Namespace
                if ns == "" { ns = "(cluster-scoped)" }
                fmt.Fprintf(w, "%s\t%s\t%s\n", res.Kind, res.Name, ns)
            }
            w.Flush()

            fmt.Printf("\n👉 Recommendation: Run 'apply -i %s -f %s --replaces %s'\n", receiptID+"-new", manifestPath, receiptID)
        },
    }

	var inspectCmd = &cobra.Command{
        Use:   "inspect",
        Short: "View resources contained within a receipt",
        // No positional args allowed now, must use -i
        Args:  cobra.NoArgs,
        Run: func(cmd *cobra.Command, args []string) {
            if receiptID == "" {
                log.Fatal("❌ Error: --id (-i) is required to inspect a receipt")
            }
            
            receipt, err := engine.LoadReceipt(receiptID)
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
                if ns == "" { ns = "(cluster-scoped)" }
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

            w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
            fmt.Fprintln(w, "ID\tRESOURCES\tAGE")

            for _, s := range summaries {
                age := time.Since(s.AppliedAt).Round(time.Second).String()
                fmt.Fprintf(w, "%s\t%d\t%s\n", s.ID, s.ResourceCount, age)
            }
            w.Flush()
        },
    }

    var cleanupCmd = &cobra.Command{
        Use:   "cleanup",
        Short: "Prune old archived receipts while retaining the most recent versions",
        Args:  cobra.NoArgs, 
        Run: func(cmd *cobra.Command, args []string) {
            // 1. Contextual Confirmation Message
            if !forceCleanup {
                if receiptID != "" {
                    fmt.Printf("⚠️  Targeting ID: '%s'. This will retain only the %d most recent versions for this resource. Continue? (y/N): ", receiptID, keepCount)
                } else {
                    fmt.Printf("⚠️  GLOBAL CLEANUP: This will retain only the %d most recent versions for ALL resources. Continue? (y/N): ", keepCount)
                }

                var response string
                fmt.Scanln(&response)
                if strings.ToLower(response) != "y" {
                    fmt.Println("❌ Cleanup cancelled.")
                    return
                }
            }

            // 2. Execute the engine logic (Pass the receiptID flag)
            // Ensure your engine.CleanupArchives signature is updated to: CleanupArchives(keep int, targetID string)
            err := engine.CleanupArchives(keepCount, receiptID)
            if err != nil {
                log.Fatalf("❌ Cleanup failed: %v", err)
            }
            
            // 3. Tailored Success Message
            if receiptID != "" {
                fmt.Printf("✨ Success! Pruned archives for '%s', keeping the %d latest versions.\n", receiptID, keepCount)
            } else {
                fmt.Printf("✨ Success! Global cleanup complete. Kept the %d latest versions for each tracked ID.\n", keepCount)
            }
        },
    }
    
    var applyCmd = &cobra.Command{
        Use:   "apply",
        Short: "Record a receipt and suggest tailored kubectl commands",
        Run: func(cmd *cobra.Command, args []string) {
            if manifestPath == "" || receiptID == "" {
                log.Fatal("❌ Error: --file and --id are required")
            }

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
                Namespace: namespace,
            }

            if replacesID != "" {
                fmt.Printf("🔄 Archiving old receipt '%s'...\n", replacesID)
                if err := engine.ArchiveReceipt(replacesID); err != nil {
                    log.Printf("⚠️  Note: Could not archive %s: %v", replacesID, err)
                }
            }

            if err := engine.SaveReceipt(receipt, replacesID); err != nil {
                log.Fatalf("❌ Save failed: %v", err)
            }

            fmt.Println("\n------------------------------------------------")
            fmt.Printf("✅ Receipt '%s' recorded (%d resources).\n", receiptID, len(resources))
            fmt.Println("👉 Next steps to deploy:")

            if namespace != "" {
                checkNs := exec.Command("kubectl", "get", "namespace", namespace)
                nsExists := checkNs.Run() == nil

                if !nsExists {
                    fmt.Printf("⚠️  Namespace '%s' does not exist yet.\n", namespace)
                    fmt.Println("   Run this first:")
                    fmt.Printf("   \033[1;33mkubectl create namespace %s\033[0m\n\n", namespace)
                    
                    fmt.Println("   Then apply your manifest:")
                    fmt.Printf("   \033[1;32mkubectl apply -f %s -n %s\033[0m\n", manifestPath, namespace)
                } else {
                    fmt.Printf("✨ Namespace '%s' is already present.\n", namespace)
                    fmt.Println("   Run the apply command:")
                    fmt.Printf("   \033[1;32mkubectl apply -f %s -n %s\033[0m\n", manifestPath, namespace)
                }
            } else {
                fmt.Printf("   \033[1;32mkubectl apply -f %s\033[0m\n", manifestPath)
            }
            fmt.Println("------------------------------------------------")
        },
    }

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
                log.Printf("⚠️  Purge finished with warnings. Active receipt preserved.")
            } else {
                err := engine.MoveToDeleted(receiptID)
				if err != nil {
					log.Printf("⚠️  Purge succeeded but moving to deleted folder failed: %v", err)
				} else {
					fmt.Printf("✨ Purge complete! Record moved to ~/.kubepurge/deleted/\n")
				}
            }

            if receipt.Namespace != "" {
                fmt.Println("\n💡 Tip: The namespace '" + receipt.Namespace + "' still exists.")
                fmt.Println("   If you want to delete it, run:")
                fmt.Printf("   \033[1;31mkubectl delete namespace %s\033[0m\n", receipt.Namespace)
            }
        },
    }

    applyCmd.Flags().StringVarP(&manifestPath, "file", "f", "", "YAML file or URL")
    applyCmd.Flags().StringVarP(&receiptID, "id", "i", "", "Unique ID for the receipt")
    applyCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Target namespace")
    applyCmd.Flags().StringVarP(&replacesID, "replaces", "r", "", "The ID of the version you are overwriting")

    purgeCmd.Flags().StringVarP(&receiptID, "id", "i", "", "ID of the receipt to purge")

    cleanupCmd.Flags().IntVarP(&keepCount, "keep", "k", 5, "Number of recent archives to retain")
    cleanupCmd.Flags().StringVarP(&receiptID, "id", "i", "", "Target a specific Resource ID for cleanup")
    cleanupCmd.Flags().BoolVarP(&forceCleanup, "force", "y", false, "Skip confirmation prompt")

    diffCmd.Flags().StringVarP(&manifestPath, "file", "f", "", "Manifest file or URL to compare")
    
	statusCmd.Flags().StringVarP(&receiptID, "id", "i", "", "ID of the receipt to check")

	inspectCmd.Flags().StringVarP(&receiptID, "id", "i", "", "ID of the receipt to inspect")

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