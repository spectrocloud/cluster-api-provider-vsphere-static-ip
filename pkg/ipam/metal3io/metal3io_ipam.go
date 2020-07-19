package metal3io

import (
	"context"
	"fmt"
	"strings"

	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Metal3Ipam struct {
	client.Client
	log logr.Logger
}

func NewIpam(cli client.Client, log logr.Logger) ipam.IPAddressManager {
	return &Metal3Ipam{
		Client: cli,
		log:    log,
	}
}

func (m Metal3Ipam) GetStaticIp(cluster *capi.Cluster, objName string) (*ipamv1.IPAddress, error) {
	if cluster == nil {
		return nil, fmt.Errorf("invalid cluster, failed to get static IP")
	}

	ipAddressList := ipamv1.IPAddressList{}
	if err := m.List(context.Background(), &ipAddressList, client.InNamespace(cluster.Namespace)); err != nil {
		m.log.V(0).Info(fmt.Sprintf("Error fetching IPAddressList: %v", err))
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}

	//the namePrefix field in the IPPool is set to the cluster-name
	//the names of IPAddresses will be prefixed with the 'namePrefix' set in IPPool
	for _, ip := range ipAddressList.Items {
		if strings.HasPrefix(ip.Name, cluster.Name) &&
			ip.Spec.Pool.Name == cluster.Name &&
			ip.Spec.Claim.Name == objName {
			m.log.V(0).Info(fmt.Sprintf("IPAddress for %s, is %s", objName, ip.Spec.Address))
			ipAddress := &ip
			return ipAddress, nil
		}
	}

	m.log.V(0).Info(fmt.Sprintf("no static IP available for %s", objName))
	return nil, nil

}
