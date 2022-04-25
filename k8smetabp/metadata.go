package k8smetabp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var baseMetadataVariables = map[string]string{
	"baseplateK8sNodeName":  "BASEPLATE_K8S_METADATA_NODE_NAME",
	"baseplateK8sNodeIP":    "BASEPLATE_K8S_METADATA_NODE_IP",
	"baseplateK8sPodName":   "BASEPLATE_K8S_METADATA_POD_NAME",
	"baseplateK8sPodIP":     "BASEPLATE_K8S_METADATA_POD_IP",
	"baseplateK8sNamespace": "BASEPLATE_K8S_METADATA_NAMESPACE",
}

type Metadata struct {
	BaseK8sMetadata map[string]string
	k8sClient       *kubernetes.Clientset
}

// New returns a new instance of metadata.
func New() (*Metadata, error) {
	baseK8sMetadata, err := fetchBaseMetadata()
	if err != nil {
		return nil, err
	}

	k8sClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	k8sAPIClient, err := kubernetes.NewForConfig(k8sClusterConfig)
	if err != nil {
		return nil, err
	}

	return &Metadata{
		BaseK8sMetadata: baseK8sMetadata,
		k8sClient:       k8sAPIClient,
	}, nil
}

// GetPodStatus
func (m *Metadata) GetPodStatus(ctx context.Context) (string, error) {
	pod, err := m.k8sClient.CoreV1().Pods(m.BaseK8sMetadata["baseplateK8sNamespace"]).Get(ctx, m.BaseK8sMetadata["baseplateK8sPodName"], metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if pod != nil {
		return string(pod.Status.Phase), nil
	}

	return "", errors.New("metadatabp:could not find pod")
}

// fetchBaseMetadata fetches the base k8s metadata from the environment
// this base metadata is needed for fetching further metadata via the k8s api.
func fetchBaseMetadata() (map[string]string, error) {
	baseK8sMetadata := make(map[string]string)
	for k, v := range baseMetadataVariables {
		value, exists := os.LookupEnv(v)
		if !exists || strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("metadatapb:%s base k8s metadata value not present", v)
		}
		baseK8sMetadata[k] = value
	}
	return baseK8sMetadata, nil
}
