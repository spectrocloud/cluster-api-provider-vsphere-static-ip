package ipam

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ClusterNameKey        = "cluster.x-k8s.io/cluster-name"
	ClusterNetworkNameKey = "cluster.x-k8s.io/network-name"
	// group is used to identify the pool, for eg., 'dev/test/prod' or 'team1/team2'
	ClusterIPPoolNameKey      = "cluster.x-k8s.io/ip-pool-name"
	ClusterIPPoolGroupKey     = "cluster.x-k8s.io/ip-pool-group"
	ClusterIPPoolNamespaceKey = "cluster.x-k8s.io/ip-pool-namespace"

	// comma-separated list of search domains
	SearchDomainsKey = "cluster.x-k8s.io/dns-search-domains"
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
