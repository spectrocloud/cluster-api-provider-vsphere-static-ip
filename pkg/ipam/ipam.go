package ipam

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	LabelClusterName    = "cluster.x-k8s.io/cluster-name"
	LabelClusterNetwork = "cluster.x-k8s.io/network"
	LabelClusterIPPool  = "cluster.x-k8s.io/ip-pool"
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
