package metal3io

import "github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"

type Metal3IPPool struct {
	// Name of the IP pool
	Name string

	// Namespace of the IP pool
	Namespace string

	// ClusterName is the name of the Cluster this object belongs to.
	ClusterName *string `json:"clusterName,omitempty"`

	//Pools contains the list of IP addresses pools
	Pools []ipam.Pool `json:"pools,omitempty"`

	// PreAllocations contains the preallocated IP addresses
	PreAllocations map[string]ipam.IPAddressStr `json:"preAllocations,omitempty"`

	// +kubebuilder:validation:Maximum=128
	// Prefix is the mask of the network as integer (max 128)
	Prefix int `json:"prefix,omitempty"`

	// Gateway is the gateway ip address
	Gateway *ipam.IPAddressStr `json:"gateway,omitempty"`

	// DNSServers is the list of dns servers
	DNSServers []ipam.IPAddressStr `json:"dnsServers,omitempty"`

	// +kubebuilder:validation:MinLength=1
	// namePrefix is the prefix used to generate the IPAddress object names
	NamePrefix string `json:"namePrefix"`
}

func NewIPPool(name, namespace, namePrefix string, clusterName *string, pools []ipam.Pool,
	preAllocations map[string]ipam.IPAddressStr, prefix int, gateway *ipam.IPAddressStr,
	dnsServers []ipam.IPAddressStr) ipam.IPPool {
	return &Metal3IPPool{
		Name:           name,
		Namespace:      namespace,
		NamePrefix:     namePrefix,
		ClusterName:    clusterName,
		Pools:          pools,
		PreAllocations: preAllocations,
		Prefix:         prefix,
		Gateway:        gateway,
		DNSServers:     dnsServers,
	}
}

func (m Metal3IPPool) GetName() string {
	return m.Name
}

func (m Metal3IPPool) GetNamespace() string {
	return m.Namespace
}

func (m Metal3IPPool) GetClusterName() (*string, error) {
	return m.ClusterName, nil
}

func (m Metal3IPPool) GetPools() ([]ipam.Pool, error) {
	return m.Pools, nil
}

func (m Metal3IPPool) GetPreAllocations() (map[string]ipam.IPAddressStr, error) {
	return m.PreAllocations, nil
}

func (m Metal3IPPool) GetPrefix() (int, error) {
	return m.Prefix, nil
}

func (m Metal3IPPool) GetGateway() (*ipam.IPAddressStr, error) {
	return m.Gateway, nil
}

func (m Metal3IPPool) GetDNSServers() ([]ipam.IPAddressStr, error) {
	return m.DNSServers, nil
}

func (m Metal3IPPool) GetNamePrefix() (string, error) {
	return m.NamePrefix, nil
}

type Metal3Pool struct {
	// Start is the first ip address that can be rendered
	Start *ipam.IPAddressStr `json:"start,omitempty"`

	// End is the last IP address that can be rendered. It is used as a validation
	// that the rendered IP is in bound.
	End *ipam.IPAddressStr `json:"end,omitempty"`

	// Subnet is used to validate that the rendered IP is in bounds. In case the
	// Start value is not given, it is derived from the subnet ip incremented by 1
	// (`192.168.0.1` for `192.168.0.0/24`)
	Subnet *ipam.IPSubnetStr `json:"subnet,omitempty"`

	// +kubebuilder:validation:Maximum=128
	// Prefix is the mask of the network as integer (max 128)
	Prefix int `json:"prefix,omitempty"`

	// Gateway is the gateway ip address
	Gateway *ipam.IPAddressStr `json:"gateway,omitempty"`

	// DNSServers is the list of dns servers
	DNSServers []ipam.IPAddressStr `json:"dnsServers,omitempty"`
}

func NewPool(start, end, gateway *ipam.IPAddressStr, subnet *ipam.IPSubnetStr, prefix int, dnsServers []ipam.IPAddressStr) ipam.Pool {
	return &Metal3Pool{
		Start:      start,
		End:        end,
		Gateway:    gateway,
		Subnet:     subnet,
		Prefix:     prefix,
		DNSServers: dnsServers,
	}
}

func (m Metal3Pool) GetStart() (*ipam.IPAddressStr, error) {
	return m.Start, nil
}

func (m Metal3Pool) GetEnd() (*ipam.IPAddressStr, error) {
	return m.End, nil
}

func (m Metal3Pool) GetSubnet() (*ipam.IPSubnetStr, error) {
	return m.Subnet, nil
}

func (m Metal3Pool) GetPrefix() (int, error) {
	return m.Prefix, nil
}

func (m Metal3Pool) GetGateway() (*ipam.IPAddressStr, error) {
	return m.Gateway, nil
}

func (m Metal3Pool) GetDNSServers() ([]ipam.IPAddressStr, error) {
	return m.DNSServers, nil
}
