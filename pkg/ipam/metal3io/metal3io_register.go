package metal3io

import (
	"fmt"

	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam/factory"
)

func init() {
	fmt.Println("register IPAM metal3io")
	factory.Register(ipam.IpamTypeMetal3io, NewIpam)
}
