package ipam

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type IPAddressManager interface {
	// gets the existing static IP for the resource
	GetIP(name string, key ObjectKey, ownerObj runtime.Object) (IPAddress, error)

	// creates/requests a new static IP for the resource if it does not exist
	AllocateIP(name string, key ObjectKey, ownerObj runtime.Object, opts ...CreateOption) error
	//ReleaseIP(cluster *capi.Cluster, owner runtime.Object, ipName string) error
}

type IPAddress interface {
	// gets the ip address name
	GetName() string

	// gets the reference to the ip claim to generate the ip address, if any
	GetClaim() (*corev1.ObjectReference, error)

	// gets the reference to the ip pool from which the ip address is generated
	GetPool() (corev1.ObjectReference, error)

	// gets the mask of the network as integer (max 128)
	GetMask() (int, error)

	// gets the gateway ip address
	GetGateway() (IPAddressStr, error)

	// gets ip address
	GetAddress() (IPAddressStr, error)
}
