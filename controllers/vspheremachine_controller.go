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
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	_ "github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam/metal3io"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
)

// VSphereMachineReconciler reconciles a VSphereMachine object
type VSphereMachineReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

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

func (r *VSphereMachineReconciler) reconcileVSphereMachineIPAddress(cluster *capi.Cluster, vsphereMachine *infrav1.VSphereMachine) (*ctrl.Result, error) {
	log := r.Log

	if vsphereMachine == nil {
		log.V(0).Info("invalid VSphereMachine, skipping reconcile IPAddress")
		return &ctrl.Result{}, nil
	}

	devices := vsphereMachine.Spec.VirtualMachineCloneSpec.Network.Devices
	log.V(0).Info(fmt.Sprintf("reconcile IP address for VSphereMachine %s", vsphereMachine.Name))
	if len(devices) == 0 {
		log.V(0).Info(fmt.Sprintf("no network device found for VSphereMachine %s", vsphereMachine.Name))
		return &ctrl.Result{}, nil
	}

	if util.IsMachineIPAllocationDHCP(devices) {
		log.V(0).Info(fmt.Sprintf("VSphereMachine %s has allocation type DHCP", vsphereMachine.Name))
		return &ctrl.Result{}, nil
	}

	updatedDevices := []infrav1.NetworkDeviceSpec{}
	dataPatch := client.MergeFrom(vsphereMachine.DeepCopy())

	newIpamFunc, ok := factory.IpamFactory[ipam.IpamTypeMetal3io]
	if !ok {
		log.V(0).Info("ipam type not supported")
		return &ctrl.Result{}, nil
	}

	ipamFunc := newIpamFunc(r.Client, log)
	devCounter := 0

	for _, dev := range devices {
		if util.IsDeviceIPAllocationDHCP(dev) || len(dev.IPAddrs) > 0 {
			updatedDevices = append(updatedDevices, dev)
			continue
		}

		ipPool, err := ipamFunc.GetAvailableIPPool(cluster, dev.NetworkName)
		if err != nil {
			log.Error(err, "failed to get an available IPPool")
			return &ctrl.Result{}, nil
		}
		if ipPool == nil {
			log.V(0).Info("waiting for IPPool to be available")
			return &ctrl.Result{}, nil
		}

		ipName := util.GetFormattedClaimName(vsphereMachine.Name, devCounter)
		ip, err := ipamFunc.GetIP(ipName, ipPool)
		if err != nil {
			return &ctrl.Result{}, errors.Wrapf(err, "failed to get allocated IP address for VSphereMachine %s", vsphereMachine.Name)
		}

		if ip == nil {
			if _, err := ipamFunc.AllocateIP(ipName, ipPool, vsphereMachine); err != nil {
				return &ctrl.Result{}, errors.Wrapf(err, "failed to allocate IP address for VSphereMachine: %s", vsphereMachine.Name)
			}

			log.V(0).Info(fmt.Sprintf("waiting for IP address to be available for the VSphereMachine %s", vsphereMachine.Name))
			return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		if err := util.ValidateIP(ip); err != nil {
			return &ctrl.Result{}, errors.Wrapf(err, "invalid IP address retrieved for VSphereMachine: %s", vsphereMachine.Name)
		}

		log.V(0).Info(fmt.Sprintf("static IP for %s is %s", vsphereMachine.Name, ip.GetName()))

		//capv expects static-ip in the CIDR format
		ipCidr := fmt.Sprintf("%s/%d", util.GetAddress(ip), util.GetMask(ip))
		log.V(0).Info(fmt.Sprintf("assigning IP address %s to VSphereMachine %s", util.GetAddress(ip), vsphereMachine.Name))

		dev.IPAddrs = []string{ipCidr}
		gateway := util.GetGateway(ip)
		//TODO: handle ipv6
		//gateway4 is required if DHCP4 is disabled, gateway6 is required if DHCP6 is disabled
		dev.Gateway4 = gateway
		dev.Nameservers = util.GetDnsServers(ip)

		updatedDevices = append(updatedDevices, dev)
		devCounter++
	}

	vsphereMachine.Spec.VirtualMachineCloneSpec.Network.Devices = updatedDevices
	if err := r.Patch(context.TODO(), vsphereMachine.DeepCopyObject(), dataPatch); err != nil {
		return &ctrl.Result{}, errors.Wrapf(err, "failed to patch VSphereMachine %s", vsphereMachine.Name)
	}

	log.V(0).Info("successfully reconciled IP address for VSphereMachine")

	return &ctrl.Result{}, nil
}

func (r *VSphereMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.VSphereMachine{}).
		Complete(r)
}
