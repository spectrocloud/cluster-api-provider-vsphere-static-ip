package metal3io

import (
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
	"k8s.io/apimachinery/pkg/types"
)

func convertToMetal3ioIP(mIP ipamv1.IPAddress, searchDomains []string) ipam.IPAddress {
	return NewIP(mIP, searchDomains)
}

func convertToMetal3ioIPPool(poolKey types.NamespacedName, mIPPool ipamv1.IPPool, searchDomains []string) ipam.IPPool {
	s := mIPPool.Spec
	preAllocations := map[string]ipam.IPAddressStr{}
	for k, v := range s.PreAllocations {
		preAllocations[k] = convertToIpamAddressStr(&v)
	}

	return NewIPPool(mIPPool, searchDomains)
}

func convertToMetal3ioPoolArray(pArr []ipamv1.Pool) []ipam.Pool {
	ipamPoolArr := []ipam.Pool{}
	for _, p := range pArr {
		ipamIp := convertToMetal3ioPool(p)
		ipamPoolArr = append(ipamPoolArr, ipamIp)
	}

	return ipamPoolArr
}

func convertToMetal3ioPool(mPool ipamv1.Pool) ipam.Pool {
	return NewPool(mPool)
}

func convertToIpamAddressStrArray(sArr []ipamv1.IPAddressStr) []ipam.IPAddressStr {
	ipamIpArr := []ipam.IPAddressStr{}
	for _, s := range sArr {
		ipamIp := convertToIpamAddressStr(&s)
		ipamIpArr = append(ipamIpArr, ipamIp)
	}

	return ipamIpArr
}

func convertToIpamAddressStr(s *ipamv1.IPAddressStr) ipam.IPAddressStr {
	if s == nil {
		return ""
	}

	return ipam.IPAddressStr(*s)
}

func convertToIpamSubnetStr(s *ipamv1.IPSubnetStr) ipam.IPSubnetStr {
	if s == nil {
		return ""
	}

	return ipam.IPSubnetStr(*s)
}
