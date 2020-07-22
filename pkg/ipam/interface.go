package ipam

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type IPAddressManager interface {
	// gets the allocated static ip by name
	GetIP(name string, poolKey ObjectKey) (IPAddress, error)

	// creates/requests a new static ip for the resource, if it does not exist
	// source ip pool is fetched using optional poolSelector, default is using poolKey
	AllocateIP(name string, poolKey ObjectKey, ownerObj runtime.Object, poolSelector *metav1.LabelSelector) (IPAddress, error)

	// releases static ip back to the ip pool
	DeallocateIP(name string, poolKey ObjectKey, ownerObj runtime.Object) error
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
