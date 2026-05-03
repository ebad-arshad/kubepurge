package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/ebad-arshad/kubepurge/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func GetGVR(kind string) (schema.GroupVersionResource, error) {
	switch kind {
	case "Deployment":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, nil
	case "Service":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}, nil
	case "ConfigMap":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, nil
	case "Secret":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, nil
	case "Namespace":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}, nil
	case "Pod":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}, nil
	case "ClusterRole":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}, nil
	case "ClusterRoleBinding":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"}, nil
	case "Role":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}, nil
	case "RoleBinding":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}, nil
	case "CustomResourceDefinition":
		return schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}, nil
    case "DaemonSet":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, nil
    case "ServiceAccount":
        return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"}, nil
	case "PodDisruptionBudget":
		return schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}, nil
	case "Ingress":
		return schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, nil
	case "StatefulSet":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, nil
	case "Job":
		return schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, nil
	case "CronJob":
		return schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}, nil
	case "StorageClass":
		return schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"}, nil
	case "PersistentVolumeClaim":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}, nil
	case "NetworkPolicy":
        return schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}, nil
    case "HorizontalPodAutoscaler":
        return schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"}, nil
    case "PriorityClass":
        return schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"}, nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("unsupported kind: %s", kind)
	}
}

// PurgeResources uses the Dynamic Client to delete resources regardless of their Type
func PurgeResources(client dynamic.Interface, resources []types.Resource) error {
	// Iterate backwards (Reverse order of installation) 
	// This ensures we delete Pods before ServiceAccounts, etc.
	for i := len(resources) - 1; i >= 0; i-- {
		res := resources[i]

		group, version := splitAPIVersion(res.APIVersion)
		
		gvr := schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: pluralize(res.Kind),
		}

		fmt.Printf("🗑️  Purging %s: %s [%s]...\n", res.Kind, res.Name, res.Namespace)

		// Dynamic client allows us to delete resources even if we don't have their Go structs
		err := client.Resource(gvr).Namespace(res.Namespace).Delete(
			context.TODO(),
			res.Name,
			metav1.DeleteOptions{},
		)

		if err != nil {
			// We don't stop the whole loop if one thing fails; 
			// it might have been deleted already.
			fmt.Printf("⚠️  Warning: Could not delete %s: %v\n", res.Name, err)
		} else {
			fmt.Printf("✅ Deleted %s\n", res.Name)
		}
	}
	return nil
}

// splitAPIVersion correctly handles "v1" vs "apps/v1"
func splitAPIVersion(apiVersion string) (group, version string) {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 1 {
		// Core resources like Pods, ConfigMaps, Services
		return "", parts[0]
	}
	// Resources like apps/v1, rbac.authorization.k8s.io/v1
	return parts[0], parts[1]
}

// pluralize converts "Deployment" to "deployments"
// Note: This is a simple version. For a real tool, one might use 
// the K8s Discovery Client to find the exact plural name.
func pluralize(kind string) string {
    k := strings.ToLower(kind)
    switch k {
    case "ingress":
        return "ingresses"
    case "priorityclass":
        return "priorityclasses"
    // Add other outliers here if they pop up
    }

    if strings.HasSuffix(k, "y") {
        return k[:len(k)-1] + "ies"
    }
    // Most resources just need an 's'
    return k + "s"
}