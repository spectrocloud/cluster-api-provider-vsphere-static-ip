package util

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsMachineIPAllocationDHCP(devices []infrav1.NetworkDeviceSpec) bool {
	isDHCP := true
	for _, dev := range devices {
		if !dev.DHCP4 && !dev.DHCP6 {
			isDHCP = false
		}
	}

	return isDHCP
}

func IsDeviceIPAllocationDHCP(device infrav1.NetworkDeviceSpec) bool {
	if device.DHCP4 || device.DHCP6 {
		return true
	}

	return false
}

func ReconcileIPClaim(cli client.Client, cluster *capi.Cluster, ownerObj runtime.Object, log logr.Logger) error {
	o := GetObjRef(ownerObj)
	claimName := o.Name

	if cluster == nil {
		fmt.Errorf("invalid cluster, failed to reconcile IPClaim")
	}

	//check if ipclaim already exists
	ic := &ipamv1.IPClaim{}
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: claimName}
	if err := cli.Get(context.Background(), key, ic); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	if ic.Name != "" {
		log.V(0).Info(fmt.Sprintf("IPClaim already exists for %s, skipping creation", claimName))
		return nil
	}

	//create a new ip claim
	return CreateIPClaim(cli, cluster, ownerObj, log)

}

func CreateIPClaim(cli client.Client, cluster *capi.Cluster, ownerObj runtime.Object, log logr.Logger) error {
	//set owner name as the claim name
	o := GetObjRef(ownerObj)
	claimName := o.Name

	ipPool := &ipamv1.IPPool{}
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name}
	if err := cli.Get(context.Background(), key, ipPool); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(0).Info("waiting for IPPool to be available, requeue the reconcile")
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
			Pool: GetObjRef(ipPool),
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

func IgnoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func GetObjRef(obj runtime.Object) corev1.ObjectReference {
	m, err := meta.Accessor(obj)
	if err != nil {
		return corev1.ObjectReference{}
	}

	v, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	return corev1.ObjectReference{
		APIVersion: v,
		Kind:       kind,
		Namespace:  m.GetNamespace(),
		Name:       m.GetName(),
		UID:        m.GetUID(),
	}
}
