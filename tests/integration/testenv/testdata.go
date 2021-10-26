package testenv

import (
	"fmt"
	"io/ioutil"

	"github.com/ghodss/yaml"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha4"
)

type TestData struct {
	M3IpamIPPool           *ipamv1.IPPool
	VSphereMachineTemplate *infrav1.VSphereMachineTemplate
	VSphereMachine         *infrav1.VSphereMachine
	VSphereCluster         *infrav1.VSphereCluster
	KubeadmControlPlane    *v1alpha4.KubeadmControlPlane
	Cluster                *capi.Cluster
	Machine                *capi.Machine
	MachineDeployment      *capi.MachineDeployment
}

func GetTestData() (*TestData, error) {
	td := &TestData{
		M3IpamIPPool:           &ipamv1.IPPool{},
		VSphereMachineTemplate: &infrav1.VSphereMachineTemplate{},
		VSphereMachine:         &infrav1.VSphereMachine{},
		VSphereCluster:         &infrav1.VSphereCluster{},
		KubeadmControlPlane:    &v1alpha4.KubeadmControlPlane{},
		Cluster:                &capi.Cluster{},
		Machine:                &capi.Machine{},
		MachineDeployment:      &capi.MachineDeployment{},
	}
	all := map[string]interface{}{
		"m3ipam_ip_pool.yaml":           td.M3IpamIPPool,
		"vsphere_machine_template.yaml": td.VSphereMachineTemplate,
		"vsphere_machine.yaml":          td.VSphereMachine,
		"vsphere_cluster.yaml":          td.VSphereCluster,
		"kcp.yaml":                      td.KubeadmControlPlane,
		"capi_cluster.yaml":             td.Cluster,
		"capi_machine.yaml":             td.Machine,
		"capi_machinedeployment.yaml":   td.MachineDeployment,
	}

	for file, v := range all {
		data, err := ioutil.ReadFile("testenv/testdata/" + file)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		if err = yaml.Unmarshal(data, v); err != nil {
			fmt.Println(err)
			return nil, err
		}
	}

	return td, nil
}
