package util

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsIPAllocationDHCP(devices []infrav1.NetworkDeviceSpec) bool {
	for _, dev := range devices {
		if dev.DHCP4 || dev.DHCP6 {
			return true
		}
	}

	return false
}

func GetStaticIp(cli client.Client, cluster *capi.Cluster, objName string, log logr.Logger) (*ipamv1.IPAddress, error) {
	if cluster != nil {
		ipAddressList := ipamv1.IPAddressList{}
		if err := cli.List(context.Background(), &ipAddressList, client.InNamespace(cluster.Namespace)); err != nil {
			log.V(0).Info(fmt.Sprintf("Error fetching IPAddressList: %v", err))
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
		}

		if len(ipAddressList.Items) > 0 {
			//the name of the IPAddress will be prefixed with the 'namePrefix' set in IPPool
			//the namePrefix of the IPPool is set to the cluster-name
			for i := range ipAddressList.Items {
				ip := ipAddressList.Items[i]
				if strings.HasPrefix(ip.Name, cluster.Name) &&
					ip.Spec.Pool.Name == cluster.Name && ip.Spec.Claim.Name == objName {
					log.V(0).Info(fmt.Sprintf("IPAddress for %s, is %s", objName, ip.Spec.Address))
					ipAddress := &ip
					return ipAddress, nil
				}
			}
		}
	}

	log.V(0).Info(fmt.Sprintf("no static ip available for %s", objName))
	return nil, nil

}

func ReconcileIPClaim(cli client.Client, cluster *capi.Cluster, claimName string, log logr.Logger) error {
	if cluster != nil {
		ipPool := &ipamv1.IPPool{}
		key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name}
		if err := cli.Get(context.Background(), key, ipPool); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(0).Info("waiting for IPPool to be available, requeue the reconcile")
				return nil
			}
		}

		//create a new ip claim
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

		if err := cli.Create(context.Background(), ipclaim); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				log.V(0).Info(fmt.Sprintf("failed to create ipclaim for %s", ipclaim.Name))
				return errors.Wrapf(err, "failed to create ipclaim for %s", ipclaim.Name)
			}
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
	}
}
