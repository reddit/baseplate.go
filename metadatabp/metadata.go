package metadatabp

import (
	"os"
	"strings"

	"github.com/reddit/baseplate.go/log"
)

type BaseMetadata string

const (
	BaseplateK8sNodeName  BaseMetadata = "baseplateK8sNodeName"
	BaseplateK8sNodeIP    BaseMetadata = "baseplateK8sNodeIP"
	BaseplateK8sPodName   BaseMetadata = "baseplateK8sPodName"
	BaseplateK8sPodIP     BaseMetadata = "baseplateK8sPodIP"
	BaseplateK8sNamespace BaseMetadata = "baseplateK8sNamespace"
)

var baseMetadataVariables = map[BaseMetadata]string{
	BaseplateK8sNodeName:  "BASEPLATE_K8S_METADATA_NODE_NAME",
	BaseplateK8sNodeIP:    "BASEPLATE_K8S_METADATA_NODE_IP",
	BaseplateK8sPodName:   "BASEPLATE_K8S_METADATA_POD_NAME",
	BaseplateK8sPodIP:     "BASEPLATE_K8S_METADATA_POD_IP",
	BaseplateK8sNamespace: "BASEPLATE_K8S_METADATA_NAMESPACE",
}

type Config struct {
	BaseK8sMetadata map[BaseMetadata]string
}

// New returns a new instance of metadata.
func New() *Config {
	baseK8sMetadata := fetchBaseMetadata()

	return &Config{
		BaseK8sMetadata: baseK8sMetadata,
	}
}

// GetBaseMetadata returns base k8s metadata based on the key provided.
func (c *Config) GetBaseMetadata(key BaseMetadata) string {
	return c.BaseK8sMetadata[key]
}

// fetchBaseMetadata fetches the base k8s metadata from the environment
// this base metadata is needed for fetching further metadata via the k8s api.
func fetchBaseMetadata() map[BaseMetadata]string {
	baseK8sMetadata := make(map[BaseMetadata]string)
	for k, v := range baseMetadataVariables {
		value, exists := os.LookupEnv(v)
		if !exists || strings.TrimSpace(value) == "" {
			log.Warnf("metadatapb:%s base k8s metadata value not present", v)
		}
		baseK8sMetadata[k] = value
	}
	return baseK8sMetadata
}
