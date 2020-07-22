package metal3io

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/pkg/errors"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/util"
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

func (m Metal3IPAM) GetIP(ipName string, poolKey ipam.ObjectKey) (ipam.IPAddress, error) {
	m.log.V(0).Info(fmt.Sprintf("get IPAddress %s", ipName))

	ip, err := getIPAddress(m.Client, poolKey, ipName, m.log)
	if err != nil {
		return nil, err
	}

	return ip, nil
}

func (m Metal3IPAM) AllocateIP(ipName string, poolKey ipam.ObjectKey, ownerObj runtime.Object, poolSelector *metav1.LabelSelector) (ipam.IPAddress, error) {
	o := util.GetObjRef(ownerObj)
	m.log.V(0).Info(fmt.Sprintf("allocate IP %s", ipName))

	//check if ip claim already exists
	ic, err := getIPClaim(m.Client, poolKey, ipName, m.log)
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
	if err = createIPClaim(m.Client, poolKey, ipName, o, poolSelector, m.log); err != nil {
		return nil, err
	}

	return nil, nil
}

func (m Metal3IPAM) DeallocateIP(ipName string, key ipam.ObjectKey, ownerObj runtime.Object) error {
	return nil
}

func getIPAddress(cli client.Client, key ipam.ObjectKey, ipName string, log logr.Logger) (ipam.IPAddress, error) {
	ic, err := getIPClaim(cli, key, ipName, log)
	if err != nil {
		log.V(0).Info(fmt.Sprintf("failed to get IPClaim %s", ipName))
		return nil, err
	}

	if ic == nil || ic.Status.Address == nil {
		log.V(0).Info(fmt.Sprintf("waiting for IPClaim %s", ipName))
		return nil, nil
	}

	ip := &ipamv1.IPAddress{}
	ipKey := types.NamespacedName{Namespace: key.Namespace, Name: ic.Status.Address.Name}
	if err := cli.Get(context.Background(), ipKey, ip); err != nil {
		return nil, errors.Wrapf(err, "failed to get IPAddress %s", ic.Status.Address.Name)
	}

	return convertToMetal3ioIP(*ip), nil
}

func getIPClaim(cli client.Client, key ipam.ObjectKey, claimName string, log logr.Logger) (*ipamv1.IPClaim, error) {
	ic := &ipamv1.IPClaim{}
	icKey := types.NamespacedName{Namespace: key.Namespace, Name: claimName}
	if err := cli.Get(context.Background(), icKey, ic); err != nil {
		return nil, util.IgnoreNotFound(err)
	}

	return ic, nil
}

func createIPClaim(cli client.Client, poolKey ipam.ObjectKey, claimName string, ownerRef v1.ObjectReference, poolSelector *metav1.LabelSelector, log logr.Logger) error {
	//set owner name as the claim name
	log.V(0).Info(fmt.Sprintf("create IPClaim %s", claimName))

	ipPool, err := getMatchingIPPool(cli, poolKey, poolSelector, log)
	if err != nil {
		return err
	}
	if ipPool == nil {
		log.V(0).Info("waiting for IPPool to be available")
		return nil
	}

	ipclaim := &ipamv1.IPClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IPClaim",
			APIVersion: ipamv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: poolKey.Namespace,
			Labels: map[string]string{
				ipam.LabelClusterName: poolKey.Name,
			},
		},
		Spec: ipamv1.IPClaimSpec{
			Pool: util.GetObjRef(ipPool),
		},
	}

	//set owner ref
	ref := metav1.OwnerReference{
		APIVersion: ipamv1.GroupVersion.String(),
		Kind:       ownerRef.Kind,
		Name:       ownerRef.Name,
		UID:        ownerRef.UID,
	}
	ownerRefs := ipclaim.GetOwnerReferences()
	ownerRefs = append(ownerRefs, ref)
	ipclaim.SetOwnerReferences(ownerRefs)

	if err := cli.Create(context.Background(), ipclaim); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create IPClaim %s", claimName)
		}
	}

	log.V(0).Info(fmt.Sprintf("created IPClaim %s, waiting for IPAddress to be available", claimName))
	return nil
}

func convertToMetal3ioIP(mIP ipamv1.IPAddress) ipam.IPAddress {
	s := mIP.Spec
	gateway := ipam.IPAddressStr("")
	if s.Gateway != nil {
		gateway = convertToIpamAddressStr(s.Gateway)
	}

	address := convertToIpamAddressStr(&s.Address)
	dnsServers := convertToIpamAddressStrArray(s.DNSServers)

	return NewIP(string(address), s.Claim, s.Pool, s.Prefix, gateway, address, dnsServers)
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

func getMatchingIPPool(cli client.Client, poolKey ipam.ObjectKey, poolSelector *metav1.LabelSelector, log logr.Logger) (*ipamv1.IPPool, error) {
	filter := map[string]string{}
	if poolSelector != nil {
		filter = poolSelector.MatchLabels
	}
	ipPools := &ipamv1.IPPoolList{}
	if err := cli.List(context.Background(), ipPools, client.InNamespace(poolKey.Namespace), client.MatchingLabels(filter)); err != nil {
		return nil, util.IgnoreNotFound(err)
	}

	if len(ipPools.Items) == 0 {
		log.V(0).Info("no matching IPPool available")
		return nil, nil
	}

	//TODO: handle selection based on IP availability
	ipPool := ipPools.Items[0]
	return &ipPool, nil
}
