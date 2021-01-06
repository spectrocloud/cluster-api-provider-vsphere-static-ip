/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam/factory"
	_ "github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam/metal3io"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VSphereMachineReconciler reconciles a VSphereMachine object
type VSphereMachineReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachinetemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachines,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ippools,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ippools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses/status,verbs=get;update;patch

func (r *VSphereMachineReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("vspheremachine", req.NamespacedName)
	var res *ctrl.Result
	var err error

	vsphereMachine := &infrav1.VSphereMachine{}
	if err := r.Get(ctx, req.NamespacedName, vsphereMachine); err != nil {
		return ctrl.Result{}, util.IgnoreNotFound(err)
	}

	// fetch the capi machine.
	machine, err := clusterutilv1.GetOwnerMachine(ctx, r.Client, vsphereMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.V(0).Info("waiting for machine controller to set ownerRef on VSphereMachine")
		return ctrl.Result{}, nil
	}

	// fetch the capi cluster
	cluster, err := clusterutilv1.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.V(0).Info("machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	res, err = r.reconcileVSphereMachineIPAddress(cluster, vsphereMachine)
	if err != nil {
		log.Error(err, "failed to reconcile VSphereMachine IP")
	}

	if res == nil {
		res = &ctrl.Result{}
	}

	return *res, err
}

func (r *VSphereMachineReconciler) reconcileVSphereMachineIPAddress(cluster *capi.Cluster, vSphereMachine *infrav1.VSphereMachine) (*ctrl.Result, error) {
	if vSphereMachine == nil {
		r.Log.V(0).Info("invalid VSphereMachine, skipping reconcile IPAddress")
		return &ctrl.Result{}, nil
	}

	log := r.Log.WithValues("vsphereMachine", vSphereMachine.Name, "namespace", vSphereMachine.Namespace)
	devices := vSphereMachine.Spec.VirtualMachineCloneSpec.Network.Devices
	log.V(0).Info("reconcile IP address for VSphereMachine")
	if len(devices) == 0 {
		log.V(0).Info("no network device found for VSphereMachine")
		return &ctrl.Result{}, nil
	}

	if util.IsMachineIPAllocationDHCP(devices) {
		log.V(0).Info("VSphereMachine has allocation type DHCP")
		return &ctrl.Result{}, nil
	}

	dataPatch := client.MergeFrom(vSphereMachine.DeepCopy())
	newIpamFunc, ok := factory.IpamFactory[ipam.IpamTypeMetal3io]
	if !ok {
		log.V(0).Info("ipam type not supported")
		return &ctrl.Result{}, nil
	}

	ipamFunc := newIpamFunc(r.Client, log)

	for i := range devices {
		if util.IsDeviceIPAllocationDHCP(devices[i]) || len(devices[i].IPAddrs) > 0 {
			continue
		}

		poolMatchLabels := r.getIPPoolMatchLabels(r.Client, vSphereMachine, r.Log)
		ipPool, err := ipamFunc.GetAvailableIPPool(poolMatchLabels, cluster.ObjectMeta)
		if err != nil {
			log.Error(err, "failed to get an available IPPool")
			return &ctrl.Result{}, nil
		}
		if ipPool == nil {
			log.V(0).Info("waiting for IPPool to be available")
			return &ctrl.Result{}, nil
		}

		ipName := util.GetFormattedClaimName(vSphereMachine.Name, i)
		ip, err := ipamFunc.GetIP(ipName, ipPool)
		if err != nil {
			return &ctrl.Result{}, errors.Wrapf(err, "failed to get allocated IP address for VSphereMachine %s", vSphereMachine.Name)
		}

		if ip == nil {
			if _, err := ipamFunc.AllocateIP(ipName, ipPool, vSphereMachine); err != nil {
				return &ctrl.Result{}, errors.Wrapf(err, "failed to allocate IP address for VSphereMachine: %s", vSphereMachine.Name)
			}

			log.V(0).Info("waiting for IP address to be available for the VSphereMachine")
			return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		if err := util.ValidateIP(ip); err != nil {
			return &ctrl.Result{}, errors.Wrapf(err, "invalid IP address retrieved for VSphereMachine: %s", vSphereMachine.Name)
		}

		log.V(0).Info("static IP selected for VSphereMachine", "IPAddressName", ip.GetName())

		//capv expects static-ip in the CIDR format
		ipCidr := fmt.Sprintf("%s/%d", util.GetAddress(ip), util.GetMask(ip))
		log.V(0).Info("assigning IP address to VSphereMachine", "IPAddress", util.GetAddress(ip))

		devices[i].IPAddrs = []string{ipCidr}
		gateway := util.GetGateway(ip)
		//TODO: handle ipv6
		//gateway4 is required if DHCP4 is disabled, gateway6 is required if DHCP6 is disabled
		devices[i].Gateway4 = gateway

		//if configured, the values of nameservers and searchDomains from the IPPool
		//will override the default values set from the VSphereMachineTemplate
		nameservers := util.GetDNSServers(ipPool)
		if len(nameservers) > 0 {
			devices[i].Nameservers = nameservers
		}
		searchDomains := util.GetSearchDomains(ipPool)
		if len(searchDomains) > 0 {
			devices[i].SearchDomains = searchDomains
		}
	}

	if err := r.Patch(context.TODO(), vSphereMachine.DeepCopyObject(), dataPatch); err != nil {
		return &ctrl.Result{}, errors.Wrapf(err, "failed to patch VSphereMachine %s", vSphereMachine.Name)
	}

	log.V(0).Info("successfully reconciled IP address for VSphereMachine")

	return &ctrl.Result{}, nil
}

func (r *VSphereMachineReconciler) getIPPoolMatchLabels(cli client.Client, vSphereMachine *infrav1.VSphereMachine, log logr.Logger) map[string]string {
	labels := map[string]string{}

	//match labels for the IPPool are retrieved from the VSphereMachineTemplate
	vmTemplateName, ok := vSphereMachine.GetAnnotations()[capi.TemplateClonedFromNameAnnotation]
	if !ok {
		log.V(0).Info("VSphereMachine's 'cloned-from-name' annotation is empty", "VSphereMachine", vSphereMachine.Name)
		return labels
	}

	vsphereMachineTemplate := &infrav1.VSphereMachineTemplate{}
	key := types.NamespacedName{Namespace: vSphereMachine.Namespace, Name: vmTemplateName}
	if err := cli.Get(context.Background(), key, vsphereMachineTemplate); err != nil {
		log.Error(err, "failed to get VSphereMachineTemplate", "VSphereMachineTemplate", vmTemplateName)
		return labels
	}

	vmTemplateLabels := util.GetObjLabels(vsphereMachineTemplate)
	for k, v := range vmTemplateLabels {
		labels[k] = v
	}

	return labels
}

func (r *VSphereMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.VSphereMachine{}).
		Complete(r)
}
