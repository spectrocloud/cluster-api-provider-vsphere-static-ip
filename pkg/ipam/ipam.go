package ipam

import (
	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IPAddressManager interface {
	// fetches the existing static IP for the resource
	GetResourceIp(cluster *capi.Cluster, resourceName string) (*ipamv1.IPAddress, error)

	// creates a new static IP for the resource if it does not exist
	CreateResourceIP(cluster *capi.Cluster, resource runtime.Object) error
}

// function to create a new IPAM
type NewIpamFunc func(cli client.Client, log logr.Logger) IPAddressManager

type IpamType string

const (
	IpamTypeMetal3io IpamType = "metal3io"
)
