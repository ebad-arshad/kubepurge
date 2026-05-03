package engine

import (
	"context"
	"fmt"

	"github.com/ebad-arshad/kubepurge/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func CheckStatus(dynClient dynamic.Interface, resources []types.Resource) ([]string, error) {
	var statusLogs []string

	for _, res := range resources {
		gvr, err := GetGVR(res.Kind)
		if err != nil {
			statusLogs = append(statusLogs, fmt.Sprintf("❓ %s/%s: Unknown Kind", res.Kind, res.Name))
			continue
		}

		_, err = dynClient.Resource(gvr).Namespace(res.Namespace).Get(context.TODO(), res.Name, metav1.GetOptions{})
		if err != nil {
			statusLogs = append(statusLogs, fmt.Sprintf("❌ %s/%s: Missing", res.Kind, res.Name))
		} else {
			statusLogs = append(statusLogs, fmt.Sprintf("✅ %s/%s: Healthy", res.Kind, res.Name))
		}
	}

	return statusLogs, nil
}