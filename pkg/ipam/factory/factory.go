package factory

import "github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"

var IpamFactory = map[ipam.IpamType]ipam.NewIpamFunc{}

func Register(ipam ipam.IpamType, ipamfunc ipam.NewIpamFunc) {
	IpamFactory[ipam] = ipamfunc
}
