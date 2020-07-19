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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

func (m Metal3Ipam) GetResourceIp(cluster *capi.Cluster, resourceName string) (*ipamv1.IPAddress, error) {
	m.log.V(0).Info(fmt.Sprintf("get IPAddress for %s", resourceName))
	if cluster == nil {
		return nil, fmt.Errorf("invalid cluster, failed to get resource IP")
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
			ip.Spec.Claim.Name == resourceName {
			m.log.V(0).Info(fmt.Sprintf("IPAddress for %s, is %s", resourceName, ip.Spec.Address))
			ipAddress := &ip
			return ipAddress, nil
		}
	}

	m.log.V(0).Info(fmt.Sprintf("no static IP available for resource %s", resourceName))
	return nil, nil
}

func (m Metal3Ipam) CreateResourceIP(cluster *capi.Cluster, resource runtime.Object) error {
	o := util.GetObjRef(resource)
	claimName := o.Name
	m.log.V(0).Info(fmt.Sprintf("create resource IP for %s", claimName))

	if cluster == nil {
		fmt.Errorf("invalid cluster, failed to create resource IP")
	}

	//check if ipclaim already exists
	ic := &ipamv1.IPClaim{}
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: claimName}
	if err := m.Get(context.Background(), key, ic); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	if ic.Name != "" {
		m.log.V(0).Info(fmt.Sprintf("IPClaim already exists for %s, skipping creation", claimName))
		return nil
	}

	//create a new ip claim
	return CreateIPClaim(m.Client, cluster, resource, m.log)

}

func CreateIPClaim(cli client.Client, cluster *capi.Cluster, ownerObj runtime.Object, log logr.Logger) error {
	//set owner name as the claim name
	o := util.GetObjRef(ownerObj)
	claimName := o.Name
	log.V(0).Info(fmt.Sprintf("create IPClaim for %s", claimName))

	ipPool := &ipamv1.IPPool{}
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name}
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
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": cluster.Name,
			},
		},
		Spec: ipamv1.IPClaimSpec{
			Pool: util.GetObjRef(ipPool),
		},
	}

	//set owner ref
	ref := metav1.OwnerReference{
		APIVersion: ipamv1.GroupVersion.String(),
		Kind:       o.Kind,
		Name:       o.Name,
		UID:        o.UID,
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
