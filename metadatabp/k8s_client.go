package metadatabp

import (
	"context"
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// K8SClient is a convinience wrapper around k8s.io/client-go
type K8SClient struct {
	*kubernetes.Clientset
}

// NewK8SClient returns a new instance of K8SClient
func NewK8SClient() (*K8SClient, error) {
	k8sClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	k8sAPIClient, err := kubernetes.NewForConfig(k8sClusterConfig)
	if err != nil {
		return nil, err
	}

	return &K8SClient{
		k8sAPIClient,
	}, nil
}

// GetPodStatus returns the status of the pod based on the namespace and the pod.
func (k *K8SClient) GetPodStatus(ctx context.Context, namespace, podName string) (string, error) {
	pod, err := k.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if pod != nil {
		return string(pod.Status.Phase), nil
	}

	return "", errors.New("metadatabp:could not find pod")
}
