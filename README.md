# 🧹 KubePurge

> **No more ghost resources. No more orphaned CRDs. Just clean clusters.**

[![Go Version](https://img.shields.io/github/go-mod/go-version/ebad-arshad/kubepurge)](https://golang.org/doc/install)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

---

## 📖 Table of Contents
* [👻 The Problem](#-the-problem-ghost-resources)
* [💡 The Solution](#-the-solution-kubepurge)
* [✨ Key Features](#-key-features)
* [💻 Usage & Commands](#-usage--commands)
* [📋 Prerequisites](#-prerequisites)
* [🚀 Installation](#-installation)
* [⚡ Quick Start](#-quick-start-the-30-second-guide)
* [🏗 Technical Architecture](#-technical-architecture)
* [⚙️ Supported Resources & Known Limitations](#️-supported-resources--known-limitations)
* [❓ Troubleshooting & FAQ](#-troubleshooting--faq)
* [🤝 Contributing](#-contributing)
* [📄 License & Credits](#-license--credits)

---

**KubePurge** is a lightweight, local state-tracking CLI for Kubernetes. It acts as a smart assistant for your daily `kubectl` workflows, ensuring that when it's time to tear down a deployment, absolutely nothing is left behind.

## 👻 The Problem: Ghost Resources

Standard Kubernetes workflows have a fatal flaw when it comes to cleanups. Consider this classic scenario:
1. You deploy `v1` of an app with a `Deployment` and **two** `ConfigMaps`.
2. A week later, you update the manifest to `v2`, removing one of the `ConfigMaps`, and run `kubectl apply -f v2.yaml`.
3. A month later, the project is over. You run `kubectl delete -f v2.yaml`.

**The Result?** That original `v1` ConfigMap is still sitting in your cluster, orphaned forever. Because `kubectl delete -f` only deletes what is currently inside the file, your cluster slowly fills up with untracked ghost resources over time.

## 💡 The Solution: KubePurge

KubePurge solves this by acting as a **Smart Guide** and generating **Receipts**. 

Instead of applying manifests blindly, you tell KubePurge what you are about to do. It parses your YAML, validates your cluster state (like checking if the namespace exists), and saves a local receipt in `~/.kubepurge/active/`. It then provides you with the exact, safest `kubectl` command to run.

When you want to update an app, KubePurge archives the old receipt to `~/.kubepurge/archive/` and tracks the new one. When you are finally ready to destroy the app, KubePurge reads the receipt and purges *every single resource* associated with that deployment's history, safely leaving shared namespaces intact. 

**Once the purge is complete, the receipt is moved to `~/.kubepurge/deleted/`.** This ensures your `active/` directory stays clutter-free while maintaining a permanent audit log of what was removed and when.

---

## ✨ Key Features

*   🚀 **Zero Cluster Footprint:** No CRDs, no Operators, and no modifications to your cluster.
*   📜 **Receipt-Based Tracking:** Every resource is recorded locally before deployment.
*   🔄 **LIFO Purging:** Resources are deleted in reverse order (Last-In, First-Out) to handle dependencies safely.
*   🕵️ **Drift Detection:** Compare local manifests against tracked state using the `diff` command.
*   🧹 **Smart Archive Maintenance:** Keep your history clean with versioned retention.

---

## 💻 Usage & Commands

KubePurge provides several commands to help you track and manage your Kubernetes resources.

### `apply`
Record a receipt and suggest tailored `kubectl` commands.
```bash
kubepurge apply -i <receipt-id> -f <manifest.yaml> [-n namespace] [-r old-receipt-id]
```
- `-i, --id`: Unique ID for the receipt (required)
- `-f, --file`: YAML manifest file or URL (required)
- `-n, --namespace`: Target namespace
- `-r, --replaces`: ID of a previous receipt to overwrite/archive

### `purge`
Deep clean resources using a tracked receipt ID. This deletes every resource originally deployed, safely handling shared namespaces.
```bash
kubepurge purge -i <receipt-id>
```
- `-i, --id`: ID of the receipt to purge (required)

### `diff`
Compare a local manifest file against a saved active receipt to see what new resources will be added.
```bash
kubepurge diff -i <receipt-id> -f <manifest.yaml>
```
- `-i, --id`: ID of the active receipt to compare against (required)
- `-f, --file`: Manifest file or URL to compare (required)

### `status`
Check the live status of the resources tracked in a receipt directly against the Kubernetes cluster. This tells you if the tracked resources are currently running or missing from the cluster.
```bash
kubepurge status -i <receipt-id>
```
- `-i, --id`: ID of the receipt to check (required)

### `inspect`
View the metadata (creation date, resource count) and the individual resources (Kind, Name, Namespace) contained within a local receipt. This is an offline command that does not query the cluster.
```bash
kubepurge inspect -i <receipt-id>
```
- `-i, --id`: ID of the receipt to inspect (required)

### `list`
List all active receipts being tracked by KubePurge.
```bash
kubepurge list
```

### `cleanup`
Retain only a specific number of recent archived receipts per ID, deleting the rest.
```bash
kubepurge cleanup [-k number] [-i receipt-id] [-y]
```
- `-k, --keep`: Number of recent archives to retain (default: 5)
- `-i, --id`: Target a specific receipt ID for cleanup (optional)
- `-y, --force`: Skip confirmation prompt

### `version`
Print the version number of KubePurge.
```bash
kubepurge version
```

---

## 📋 Prerequisites

Before dropping KubePurge into your workflow, ensure you have the following ready:
*   **Go 1.21+** installed on your system.
*   **`kubectl`** installed and configured.
*   An active **Kubeconfig** with access to your cluster.

*Note: KubePurge is fully compatible with Linux, macOS, and WSL2.*

---

## 🚀 Installation

### Using `go install` (Recommended)
The fastest way to get KubePurge up and running is directly via Go:
```bash
go install github.com/ebad-arshad/kubepurge/cmd/kubepurge@latest
```

### Build from Source
If you prefer to get your hands dirty and build it yourself:
```bash
# 1. Clone the repository
git clone https://github.com/ebad-arshad/kubepurge.git
cd kubepurge

# 2. Build the binary
go build -o kubepurge ./cmd/kubepurge

# 3. Create the directory (if it doesn't exist)
mkdir -p ~/.local/bin

# 4. Move the binary
mv kubepurge ~/.local/bin/

# 5. Ensure it's in your PATH
export PATH="$HOME/.local/bin:$PATH"

# 6. Refresh your terminal
source ~/.bashrc

```

### 🧹 Uninstalling KubePurge
If you ever want to completely remove KubePurge and its local history, you can clean it up in seconds leaving zero footprint:
```bash
# 1. Remove the binary (if installed via build)
sudo rm /usr/local/bin/kubepurge

# Or if installed via `go install`
rm $(go env GOPATH)/bin/kubepurge

# 2. Delete your local state history
rm -rf ~/.kubepurge
```
*Note: If you just want to clear some space without uninstalling the tool, you can use the `kubepurge cleanup -k <number>` command to prune old archives! You can also manually delete files inside `~/.kubepurge/active`, `archive`, or `deleted` if you need to clear specific receipt records.*

---

## ⚡ Quick Start (The "30-Second" Guide)

Here is how to use KubePurge in four simple steps:

1. **Deploy and Track:** Tell KubePurge what you're deploying to generate a receipt.
   ```bash
   kubepurge apply -i demo -f my-app.yaml
   ```
2. **Check Live Status:** Verify your resources are running happily in the cluster.
   ```bash
   kubepurge status -i demo
   ```
3. **Inspect Local Record:** View the tracked metadata offline to see what's actually under management.
   ```bash
   kubepurge inspect -i demo
   ```
4. **Deep Clean:** Destroy the deployment completely, leaving no trace behind.
   ```bash
   kubepurge purge -i demo
   ```

---

## 🏗 Technical Architecture

KubePurge is built on a strict **"No-Footprint" philosophy**. We believe tools shouldn't clutter the clusters they're supposed to clean. All state is maintained locally in `~/.kubepurge/`:

*   📂 **`active/` (Live):** Contains lightweight JSON receipts of your currently deployed apps.
*   📂 **`archive/` (History):** Stores previous versions of your deployments when you update them (using the `--replaces` flag).
*   📂 **`deleted/` (Audit):** Keeps a permanent, timestamped log of everything you've successfully purged.

```bash
~/.kubepurge/
├── active/     # Active current "Source of Truth"
├── archive/    # Version history (for rollbacks/audit)
└── deleted/    # The graveyard (audit logs of purges)
```
**Reverse-Order Deletion (LIFO):** During a purge, KubePurge parses your receipt and deletes resources in reverse order (Last-In, First-Out). This is a critical safety feature ensuring that dependent resources (like Pods) are deleted *before* their underlying dependencies (like Namespaces or Secrets), preventing hanging resources and annoying timeout errors.

---

## ⚙️ Supported Resources & Known Limitations

KubePurge relies on basic pluralization logic to dynamically interact with the Kubernetes API. 

*   **Custom Resource Definitions (CRDs):** Most standard resources (Deployments, ConfigMaps, Ingresses, PersistentVolumes) work out of the box. However, if you are deploying CRDs with highly irregular plural names that don't follow standard English rules, KubePurge might struggle to purge them. Feel free to open a PR to add exceptions to the `pluralize()` function in `reaper.go`!
*   **Helm Integration:** KubePurge parses raw YAML. If you want to track Helm releases, simply save the template output to a file and apply it:
    ```bash
    helm template my-release ./my-chart > temp.yaml
    kubepurge apply -i my-helm-app -f temp.yaml
    ```

---

## ❓ Troubleshooting & FAQ

**"Namespace not found" when running `apply`?**
KubePurge checks if your target namespace exists before applying. If it doesn't, KubePurge will pause and print the exact `kubectl create namespace` command you need to run first.

**Cluster connectivity errors during `status` or `purge`?**
KubePurge relies on your active `KUBECONFIG` (usually `~/.kube/config`). Ensure your context is set correctly by running `kubectl config current-context` and that your cluster is actually reachable.

---

## 🤝 Contributing

We love community input! Whether you want to improve the reaper logic, add support for custom manifest parsers, or just fix a typo, your contributions are highly welcome. Feel free to open an issue or submit a Pull Request on [GitHub](https://github.com/ebad-arshad/kubepurge).

---

## 📄 License & Credits

KubePurge is open-source software licensed under the [MIT License](https://opensource.org/licenses/MIT).
