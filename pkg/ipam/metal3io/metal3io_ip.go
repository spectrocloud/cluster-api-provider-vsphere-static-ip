package metal3io

import (
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
	corev1 "k8s.io/api/core/v1"
)

type Metal3IP struct {
	// Name of the IP
	Name string `json:"name"`

	// Claim points to the object the IPClaim was created for.
	Claim corev1.ObjectReference `json:"claim"`

	// Pool is the IPPool this was generated from.
	Pool corev1.ObjectReference `json:"pool"`

	// Prefix is the mask of the network as integer (max 128)
	Prefix int `json:"prefix,omitempty"`

	// Gateway is the gateway ip address
	Gateway ipam.IPAddressStr `json:"gateway,omitempty"`

	// Address contains the IP address
	Address ipam.IPAddressStr `json:"address"`

	// DNSServers is the list of dns servers
	DNSServers []ipam.IPAddressStr `json:"dnsServers,omitempty"`
}

func NewIP(name string, claim, pool corev1.ObjectReference,
	prefix int, gateway, address ipam.IPAddressStr, dnsServers []ipam.IPAddressStr) ipam.IPAddress {
	return &Metal3IP{
		Name:       name,
		Claim:      claim,
		Pool:       pool,
		Prefix:     prefix,
		Gateway:    gateway,
		Address:    address,
		DNSServers: dnsServers,
	}
}

func (m Metal3IP) GetName() string {
	return m.Name
}

func (m Metal3IP) GetClaim() (*corev1.ObjectReference, error) {
	return &m.Claim, nil
}

func (m Metal3IP) GetPool() (corev1.ObjectReference, error) {
	return m.Pool, nil
}

func (m Metal3IP) GetMask() (int, error) {
	return m.Prefix, nil
}

func (m Metal3IP) GetGateway() (ipam.IPAddressStr, error) {
	return m.Gateway, nil
}

func (m Metal3IP) GetAddress() (ipam.IPAddressStr, error) {
	return m.Address, nil
}

func (m Metal3IP) GetDnsServers() ([]ipam.IPAddressStr, error) {
	return m.DNSServers, nil
}
