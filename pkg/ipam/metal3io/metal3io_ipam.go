package metal3io

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/pkg/errors"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/util"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Metal3IPAM struct {
	client.Client
	log logr.Logger
}

func NewIpam(cli client.Client, log logr.Logger) ipam.IPAddressManager {
	return &Metal3IPAM{
		Client: cli,
		log:    log,
	}
}

func (m Metal3IPAM) GetIP(ipName string, pool ipam.IPPool) (ipam.IPAddress, error) {
	m.log.V(0).Info(fmt.Sprintf("get IPAddress %s", ipName))

	ip, err := getIPAddress(m.Client, pool, ipName, m.log)
	if err != nil {
		return nil, err
	}

	return ip, nil
}

func (m Metal3IPAM) AllocateIP(ipName string, pool ipam.IPPool, ownerObj runtime.Object) (ipam.IPAddress, error) {
	o := util.GetObjRef(ownerObj)
	m.log.V(0).Info(fmt.Sprintf("allocate IP %s", ipName))

	//check if ip claim already exists
	ic, err := getIPClaim(m.Client, pool, ipName)
	if err != nil {
		m.log.V(0).Info(fmt.Sprintf("failed to get IPClaim %s", ipName))
		return nil, err
	}

	//if IPClaim exists, the corresponding IPAddress is expected to be generated
	if ic != nil {
		m.log.V(0).Info(fmt.Sprintf("IPClaim %s already exists, skipping creation", ipName))
		return nil, nil
	}

	//create a new ip claim
	if err = createIPClaim(m.Client, pool, ipName, o, m.log); err != nil {
		return nil, err
	}

	return nil, nil
}

func (m Metal3IPAM) DeallocateIP(name string, pool ipam.IPPool, ownerObj runtime.Object) error {
	return nil
}

func (m Metal3IPAM) GetAvailableIPPool(clusterMeta metav1.ObjectMeta, networkName string) (ipam.IPPool, error) {
	poolKey := util.GetIPPoolNamespacedName(clusterMeta)

	//labels to select the ip pool
	poolSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			ipam.ClusterIPPoolGroupKey: util.ConvertToLabelFormat(poolKey.Name),
			ipam.ClusterNetworkNameKey: util.ConvertToLabelFormat(networkName),
		},
	}

	filter := poolSelector.MatchLabels
	ipPools := &ipamv1.IPPoolList{}
	if err := m.List(context.Background(), ipPools, client.InNamespace(poolKey.Namespace), client.MatchingLabels(filter)); err != nil {
		return nil, util.IgnoreNotFound(err)
	}

	if len(ipPools.Items) == 0 {
		m.log.V(0).Info("failed to get a matching IPPool")
		return nil, nil
	}

	//TODO: refactor searchDomains, once its added in metal3io
	searchDomains := []string{}
	if sdList, ok := clusterMeta.Annotations[ipam.SearchDomainsKey]; ok {
		searchDomains = strings.Split(sdList, ",")
	}

	//TODO: handle selection based on ip address availability
	ipPool := ipPools.Items[0]
	m.log.V(0).Info(fmt.Sprintf("IPPool %s is available", ipPool.Name))

	return convertToMetal3ioIPPool(poolKey, ipPool, searchDomains), nil
}

func getIPAddress(cli client.Client, pool ipam.IPPool, ipName string, log logr.Logger) (ipam.IPAddress, error) {
	ic, err := getIPClaim(cli, pool, ipName)
	if err != nil {
		log.V(0).Info(fmt.Sprintf("failed to get IPClaim %s", ipName))
		return nil, err
	}

	if ic == nil || ic.Status.Address == nil {
		log.V(0).Info(fmt.Sprintf("waiting for IPClaim %s", ipName))
		return nil, nil
	}

	ip := &ipamv1.IPAddress{}
	ipKey := types.NamespacedName{Namespace: pool.GetNamespace(), Name: ic.Status.Address.Name}
	if err := cli.Get(context.Background(), ipKey, ip); err != nil {
		return nil, errors.Wrapf(err, "failed to get IPAddress %s", ic.Status.Address.Name)
	}

	searchDomains, err := pool.GetSearchDomains()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get search domains for %s", pool.GetName())
	}

	return convertToMetal3ioIP(*ip, searchDomains), nil
}

func getIPClaim(cli client.Client, pool ipam.IPPool, claimName string) (*ipamv1.IPClaim, error) {
	ic := &ipamv1.IPClaim{}
	icKey := types.NamespacedName{Namespace: pool.GetNamespace(), Name: claimName}
	if err := cli.Get(context.Background(), icKey, ic); err != nil {
		return nil, util.IgnoreNotFound(err)
	}

	return ic, nil
}

func createIPClaim(cli client.Client, pool ipam.IPPool, claimName string, ownerRef v1.ObjectReference, log logr.Logger) error {
	//set owner name as the claim name
	log.V(0).Info(fmt.Sprintf("create IPClaim %s", claimName))
	ipPool := &ipamv1.IPPool{}
	poolKey := types.NamespacedName{Namespace: pool.GetNamespace(), Name: pool.GetName()}
	if err := cli.Get(context.Background(), poolKey, ipPool); err != nil {
		log.V(0).Info(fmt.Sprintf("failed to get IPPool %s", pool.GetName()))
		return util.IgnoreNotFound(err)
	}

	ipclaim := &ipamv1.IPClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IPClaim",
			APIVersion: ipamv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: pool.GetNamespace(),
			Labels: map[string]string{
				ipam.ClusterNameKey: pool.GetName(),
			},
		},
		Spec: ipamv1.IPClaimSpec{
			Pool: util.GetObjRef(ipPool),
		},
	}

	//set owner ref
	ref := metav1.OwnerReference{
		APIVersion: ownerRef.APIVersion,
		Kind:       ownerRef.Kind,
		Name:       ownerRef.Name,
		UID:        ownerRef.UID,
	}
	ipclaim.SetOwnerReferences([]metav1.OwnerReference{ref})

	if err := cli.Create(context.Background(), ipclaim); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create IPClaim %s", claimName)
		}
	}

	log.V(0).Info(fmt.Sprintf("created IPClaim %s, waiting for IPAddress to be available", claimName))
	return nil
}