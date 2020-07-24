package ipam

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	LabelClusterName          = "cluster.x-k8s.io/cluster-name"
	LabelClusterNetworkName   = "cluster.x-k8s.io/network-name"
	LabelClusterIPPoolName    = "cluster.x-k8s.io/ip-pool-name"
	ClusterIPPoolNameKey      = "cluster.x-k8s.io/ip-pool-name"
	ClusterIPPoolNamespaceKey = "cluster.x-k8s.io/ip-pool-namespace"
	SearchDomainKey           = "cluster.x-k8s.io/ip-search-domain"
)

// ObjectKey identifies a Kubernetes Object.
type ObjectKey = types.NamespacedName

// function to create a new IPAM
type NewIpamFunc func(cli client.Client, log logr.Logger) IPAddressManager

type IpamType string

const (
	IpamTypeMetal3io IpamType = "metal3io"
)

type IPAddressStr string
type IPSubnetStr string
