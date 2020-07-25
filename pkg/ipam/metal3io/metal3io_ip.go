package metal3io

import (
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
	corev1 "k8s.io/api/core/v1"
)

type Metal3IP struct {
	// Name of the IP
	Name string `json:"name"`

	ipamv1.IPAddress

	// SearchDomains is a list of search domains used when resolving IP
	// addresses with DNS.
	SearchDomains []string `json:"searchDomains,omitempty"`
}

func NewIP(name string, ipAddress ipamv1.IPAddress, searchDomains []string) ipam.IPAddress {
	return &Metal3IP{
		Name:          name,
		IPAddress:     ipAddress,
		SearchDomains: searchDomains,
	}
}

func (m Metal3IP) GetName() string {
	return m.Name
}

func (m Metal3IP) GetClaim() (*corev1.ObjectReference, error) {
	return &m.Spec.Claim, nil
}

func (m Metal3IP) GetPool() (corev1.ObjectReference, error) {
	return m.Spec.Pool, nil
}

func (m Metal3IP) GetMask() (int, error) {
	return m.Spec.Prefix, nil
}

func (m Metal3IP) GetGateway() (ipam.IPAddressStr, error) {
	gateway := ipam.IPAddressStr("")
	if m.Spec.Gateway != nil {
		gateway = convertToIpamAddressStr(m.Spec.Gateway)
	}

	return gateway, nil
}

func (m Metal3IP) GetAddress() (ipam.IPAddressStr, error) {
	return convertToIpamAddressStr(&m.Spec.Address), nil
}

func (m Metal3IP) GetDnsServers() ([]ipam.IPAddressStr, error) {
	return convertToIpamAddressStrArray(m.Spec.DNSServers), nil
}

func (m Metal3IP) GetSearchDomains() ([]string, error) {
	return m.SearchDomains, nil
}
