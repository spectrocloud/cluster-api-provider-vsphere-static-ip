package ipam

import (
	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IPAddressManager interface {
	GetStaticIp(cluster *capi.Cluster, objName string) (*ipamv1.IPAddress, error)
}

// function to create a new IPAM
type NewIpamFunc func(cli client.Client, log logr.Logger) IPAddressManager

type IpamType string

const (
	IpamTypeMetal3io IpamType = "metal3io"
)
