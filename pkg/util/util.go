package util

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
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
