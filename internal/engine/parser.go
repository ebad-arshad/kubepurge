package engine

import (
	"bytes"
	"io"
	"github.com/ebad-arshad/kubepurge/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// ExtractResources now takes a targetNS to handle the '-n' flag override
func ExtractResources(rawYAML []byte, targetNS string) ([]types.Resource, error) {
	var resources []types.Resource
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(rawYAML), 4096)

	for {
		var doc map[string]interface{}
		err := decoder.Decode(&doc)
		if err == io.EOF { break }
		if err != nil { return nil, err }
		if doc == nil { continue }

		metadata := doc["metadata"].(map[string]interface{})
		
		res := types.Resource{
			Kind:       doc["kind"].(string),
			APIVersion: doc["apiVersion"].(string),
			Name:       metadata["name"].(string),
		}

		// LOGIC: Determining the Namespace
		// 1. Check if the YAML has a namespace defined
		yamlNS, hasYamlNS := metadata["namespace"].(string)

		// 2. Apply the Logic:
		// If the resource is cluster-scoped (like a CRD), we don't set a namespace.
		// If it's namespaced, the '-n' flag (targetNS) takes priority over the YAML.
		if isClusterScoped(res.Kind) {
			res.Namespace = "" // Force empty for cluster-scoped
		} else if targetNS != "" {
			res.Namespace = targetNS // Use terminal flag
		} else if hasYamlNS {
			res.Namespace = yamlNS // Use YAML value
		} else {
			res.Namespace = "default" // Fallback
		}

		resources = append(resources, res)
	}
	return resources, nil
}

// Simple helper to identify common cluster-wide resources
func isClusterScoped(kind string) bool {
	clusterScopedKinds := map[string]bool{
		"ClusterRole":              true,
		"ClusterRoleBinding":       true,
		"CustomResourceDefinition": true,
		"Namespace":                true,
		"PersistentVolume":         true,
		"StorageClass":             true,
	}
	return clusterScopedKinds[kind]
}