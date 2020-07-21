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

func (m Metal3IPAM) GetIP(ipName string, key ipam.ObjectKey, ownerObj runtime.Object) (ipam.IPAddress, error) {
	o := util.GetObjRef(ownerObj)
	claimName := o.Name
	m.log.V(0).Info(fmt.Sprintf("get IPAddress for %s", claimName))

	ip, err := getIPAddress(m.Client, key, claimName, m.log)
	if err != nil {
		return nil, util.IgnoreNotFound(err)
	}

	return ip, nil
}

func (m Metal3IPAM) AllocateIP(ipName string, key ipam.ObjectKey, ownerObj runtime.Object, opts ...ipam.CreateOption) (ipam.IPAddress, error) {
	o := util.GetObjRef(ownerObj)
	claimName := o.Name
	m.log.V(0).Info(fmt.Sprintf("allocate IP for %s", claimName))

	//check if ip address already exists
	ip, err := getIPAddress(m.Client, key, claimName, m.log)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	if ip != nil {
		m.log.V(0).Info(fmt.Sprintf("IPAddress already exists for %s, skipping creation", claimName))
		return ip, nil
	}

	//create a new ip claim
	if err = createIPClaim(m.Client, key, o, m.log); err != nil {
		return nil, err
	}

	return nil, nil
}

func (m Metal3IPAM) ReleaseIP(ipName string, key ipam.ObjectKey, ownerObj runtime.Object, opts ...ipam.DeleteOption) error {
	return nil
}

func getIPAddress(cli client.Client, key ipam.ObjectKey, claimName string, log logr.Logger) (ipam.IPAddress, error) {
	ic, err := getIPClaim(cli, key, claimName, log)
	if err != nil {
		return nil, err
	}

	if ic == nil || ic.Status.Address == nil {
		log.V(0).Info(fmt.Sprintf("waiting for IPClaim %s", claimName))
		return nil, nil
	}

	ip := &ipamv1.IPAddress{}
	ipKey := types.NamespacedName{Namespace: key.Namespace, Name: ic.Status.Address.Name}
	if err := cli.Get(context.Background(), ipKey, ip); err != nil {
		log.V(0).Info(fmt.Sprintf("failed to get IPAddress %v", err))
		return nil, err
	}

	return convertToMetal3ioIP(*ip), nil
}

func getIPClaim(cli client.Client, key ipam.ObjectKey, claimName string, log logr.Logger) (*ipamv1.IPClaim, error) {
	ic := &ipamv1.IPClaim{}
	icKey := types.NamespacedName{Namespace: key.Namespace, Name: claimName}
	if err := cli.Get(context.Background(), icKey, ic); err != nil {
		log.V(1).Info(fmt.Sprintf("failed to get IPClaim %v", claimName))
		return nil, err
	}

	return ic, nil
}

func createIPClaim(cli client.Client, key ipam.ObjectKey, ownerRef v1.ObjectReference, log logr.Logger) error {
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
			return errors.Wrapf(err, "failed to create IPClaim for %s", ipclaim.Name)
		}
	}

	log.V(0).Info(fmt.Sprintf("created IPClaim %s, waiting for IPAddress", claimName))
	return nil
}

func convertToMetal3ioIP(mIP ipamv1.IPAddress) ipam.IPAddress {
	s := mIP.Spec
	gateway := ipam.IPAddressStr("")
	if s.Gateway != nil {
		gateway = convertToIpamAddressStr(s.Gateway)
	}

	address := convertToIpamAddressStr(&s.Address)

	return NewIP(string(address), s.Claim, s.Pool, s.Prefix, gateway, address)
}

func convertToIpamAddressStr(s *ipamv1.IPAddressStr) ipam.IPAddressStr {
	if s == nil {
		return ""
	}

	return ipam.IPAddressStr(*s)
}
