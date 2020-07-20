package metal3io

import (
	"context"
	"fmt"
	"strings"

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

func (m Metal3IPAM) GetIP(ipName string, key ipam.ObjectKey, ownerObj runtime.Object) (ipam.IPAddress, error) {
	o := util.GetObjRef(ownerObj)
	resourceName := o.Name
	m.log.V(0).Info(fmt.Sprintf("get IPAddress for %s", resourceName))

	ipAddressList := ipamv1.IPAddressList{}
	if err := m.List(context.Background(), &ipAddressList, client.InNamespace(key.Namespace)); err != nil {
		m.log.V(0).Info(fmt.Sprintf("Error fetching IPAddressList: %v", err))
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}

	//the namePrefix field in the IPPool is set to the cluster-name
	//the names of IPAddresses will be prefixed with the 'namePrefix' set in IPPool
	for _, ip := range ipAddressList.Items {
		if strings.HasPrefix(ip.Name, key.Name) &&
			ip.Spec.Pool.Name == key.Name &&
			ip.Spec.Claim.Name == resourceName {
			m.log.V(0).Info(fmt.Sprintf("IPAddress for %s, is %s", resourceName, ip.Spec.Address))
			return convertToMetal3ioIP(ip), nil
		}
	}

	m.log.V(0).Info(fmt.Sprintf("no static IP available for resource %s", resourceName))
	return nil, nil
}

func convertToMetal3ioIP(mIP ipamv1.IPAddress) ipam.IPAddress {
	s := mIP.Spec
	gateway := ipam.IPAddressStr("")
	if s.Gateway != nil {
		gateway = convertToIpamAddressStr(s.Gateway)
	}
	return &Metal3IP{
		Claim:   s.Claim,
		Pool:    s.Pool,
		Prefix:  s.Prefix,
		Gateway: gateway,
		Address: convertToIpamAddressStr(&s.Address),
	}
}

func convertToIpamAddressStr(s *ipamv1.IPAddressStr) ipam.IPAddressStr {
	if s == nil {
		return ""
	}

	return ipam.IPAddressStr(*s)
}

func (m Metal3IPAM) AllocateIP(ipName string, key ipam.ObjectKey, ownerObj runtime.Object, opts ...ipam.CreateOption) error {
	o := util.GetObjRef(ownerObj)
	claimName := o.Name
	m.log.V(0).Info(fmt.Sprintf("create resource IP for %s", claimName))

	//check if ipclaim already exists
	ic := &ipamv1.IPClaim{}
	ickey := types.NamespacedName{Namespace: key.Namespace, Name: claimName}
	if err := m.Get(context.Background(), ickey, ic); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	if ic.Name != "" {
		m.log.V(0).Info(fmt.Sprintf("IPClaim already exists for %s, skipping creation", claimName))
		return nil
	}

	//create a new ip claim
	return CreateIPClaim(m.Client, key, o, m.log)

}

func CreateIPClaim(cli client.Client, key ipam.ObjectKey, ownerRef v1.ObjectReference, log logr.Logger) error {
	//set owner name as the claim name
	claimName := ownerRef.Name
	log.V(0).Info(fmt.Sprintf("create IPClaim for %s", claimName))

	ipPool := &ipamv1.IPPool{}
	if err := cli.Get(context.Background(), key, ipPool); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(0).Info("waiting for IPPool to be available")
			return nil
		}
	}

	ipclaim := &ipamv1.IPClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IPClaim",
			APIVersion: ipamv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: key.Namespace,
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": key.Name,
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
			log.V(0).Info(fmt.Sprintf("failed to create ipclaim for %s", ipclaim.Name))
			return errors.Wrapf(err, "failed to create ipclaim for %s", ipclaim.Name)
		}
	}

	return nil
}
