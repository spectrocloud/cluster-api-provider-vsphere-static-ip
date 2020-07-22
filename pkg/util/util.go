package util

import (
	"fmt"
	"strings"

	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
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

func ValidateIP(ip ipam.IPAddress) error {
	if addr, err := ip.GetAddress(); addr == "" || err != nil {
		if err != nil {
			return err
		}
		return fmt.Errorf("invalid 'address' in IPAddress")
	}
	if gat, err := ip.GetGateway(); gat == "" || err != nil {
		if err != nil {
			return err
		}
		return fmt.Errorf("invalid 'gateway' in IPAddress")
	}

	return nil
}

func GetAddress(ip ipam.IPAddress) string {
	if a, err := ip.GetAddress(); err == nil {
		return string(a)
	}
	return ""
}

func GetGateway(ip ipam.IPAddress) string {
	if g, err := ip.GetGateway(); err == nil {
		return string(g)
	}
	return ""
}

func GetMask(ip ipam.IPAddress) int {
	if m, err := ip.GetMask(); err == nil {
		return m
	}
	return 0
}

func GetDnsServers(ip ipam.IPAddress) []string {
	dnsServers := []string{}
	if dnsArr, err := ip.GetDnsServers(); err == nil {
		for _, d := range dnsArr {
			dnsServers = append(dnsServers, string(d))
		}
	}
	return dnsServers
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

func GetObjName(obj runtime.Object) string {
	o := GetObjRef(obj)
	return o.Name
}

func ConvertToLabelFormat(s string) string {
	//lowercase, replacing '-' for space
	return strings.ReplaceAll(strings.ToLower(s), " ", "-")
}

func GetFormattedClaimName(deviceName, ownerName string) string {
	return fmt.Sprintf("%s-%s", deviceName, ownerName)
}
