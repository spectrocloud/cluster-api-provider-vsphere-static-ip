package metal3io

import (
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
)

type Metal3IPPool struct {
	// Name of the IP pool
	Name string

	// Namespace of the IP pool
	Namespace string

	ipamv1.IPPool

	// SearchDomains is a list of search domains used when resolving IP
	// addresses with DNS.
	SearchDomains []string `json:"searchDomains,omitempty"`
}

func NewIPPool(name, namespace string, pool ipamv1.IPPool, searchDomains []string) ipam.IPPool {
	return &Metal3IPPool{
		Name:          name,
		Namespace:     namespace,
		IPPool:        pool,
		SearchDomains: searchDomains,
	}
}

func (m Metal3IPPool) GetName() string {
	return m.Name
}

func (m Metal3IPPool) GetNamespace() string {
	return m.Namespace
}

func (m Metal3IPPool) GetClusterName() (*string, error) {
	return m.IPPool.Spec.ClusterName, nil
}

func (m Metal3IPPool) GetPools() ([]ipam.Pool, error) {
	return convertToMetal3ioPoolArray(m.IPPool.Spec.Pools), nil
}

func (m Metal3IPPool) GetPreAllocations() (map[string]ipam.IPAddressStr, error) {
	preAllocations := map[string]ipam.IPAddressStr{}
	for k, v := range m.IPPool.Spec.PreAllocations {
		preAllocations[k] = convertToIpamAddressStr(&v)
	}

	return preAllocations, nil
}

func (m Metal3IPPool) GetPrefix() (int, error) {
	return m.IPPool.Spec.Prefix, nil
}

func (m Metal3IPPool) GetGateway() (*ipam.IPAddressStr, error) {
	gateway := ipam.IPAddressStr("")
	if m.IPPool.Spec.Gateway != nil {
		gateway = convertToIpamAddressStr(m.IPPool.Spec.Gateway)
	}

	return &gateway, nil
}

func (m Metal3IPPool) GetDNSServers() ([]ipam.IPAddressStr, error) {
	dnsServers := []ipam.IPAddressStr{}
	for _, d := range m.IPPool.Spec.DNSServers {
		dnsServers = append(dnsServers, convertToIpamAddressStr(&d))
	}

	return dnsServers, nil
}

func (m Metal3IPPool) GetNamePrefix() (string, error) {
	return m.IPPool.Spec.NamePrefix, nil
}

func (m Metal3IPPool) GetSearchDomains() ([]string, error) {
	return m.SearchDomains, nil
}

type Metal3Pool struct {
	ipamv1.Pool
}

func NewPool(pool ipamv1.Pool) ipam.Pool {
	return &Metal3Pool{
		Pool: pool,
	}
}

func (m Metal3Pool) GetStart() (*ipam.IPAddressStr, error) {
	start := ipam.IPAddressStr("")
	if m.Start != nil {
		start = convertToIpamAddressStr(m.Start)
	}

	return &start, nil
}

func (m Metal3Pool) GetEnd() (*ipam.IPAddressStr, error) {
	end := ipam.IPAddressStr("")
	if m.End != nil {
		end = convertToIpamAddressStr(m.End)
	}

	return &end, nil
}

func (m Metal3Pool) GetSubnet() (*ipam.IPSubnetStr, error) {
	subnet := ipam.IPSubnetStr("")
	if m.Subnet != nil {
		subnet = convertToIpamSubnetStr(m.Subnet)
	}
	return &subnet, nil
}

func (m Metal3Pool) GetPrefix() (int, error) {
	return m.Prefix, nil
}

func (m Metal3Pool) GetGateway() (*ipam.IPAddressStr, error) {
	gateway := ipam.IPAddressStr("")
	if m.Gateway != nil {
		gateway = convertToIpamAddressStr(m.Gateway)
	}
	return &gateway, nil
}

func (m Metal3Pool) GetDNSServers() ([]ipam.IPAddressStr, error) {
	return convertToIpamAddressStrArray(m.DNSServers), nil
}
