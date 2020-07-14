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
	"strconv"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/util"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ippools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ippools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses/status,verbs=get;update;patch

func (r *VSphereMachineReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("vspheremachine", req.NamespacedName)

	ctx := context.Background()
	log := r.Log
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
		return nil, fmt.Errorf("invalid VSphereMachine: %s", vsphereMachine.Name)
	}

	devices := vsphereMachine.Spec.VirtualMachineCloneSpec.Network.Devices
	numOfDevices := len(devices)
	log.V(0).Info(fmt.Sprintf("reconcile IP address for VSphereMachine %s, with a device count of %d", vsphereMachine.Name, numOfDevices))
	if numOfDevices == 0 {
		log.V(0).Info(fmt.Sprintf("no network device found for VSphereMachine %s", vsphereMachine.Name))
		return &ctrl.Result{}, nil
	}

	updatedDevices := []infrav1.NetworkDeviceSpec{}
	dataPatch := client.MergeFrom(vsphereMachine.DeepCopy())

	for _, dev := range devices {
		if util.IsIPAllocationDHCP(dev) || len(dev.IPAddrs) > 0 {
			updatedDevices = append(updatedDevices, dev)
			continue
		}

		ipAddr, err := util.GetStaticIp(r.Client, cluster, vsphereMachine.Name, r.Log)
		if err != nil {
			return &ctrl.Result{}, err
		}

		if ipAddr == nil {
			//if ip address list is not found, create a new ip claim
			if err := util.ReconcileIPClaim(r.Client, cluster, vsphereMachine, r.Log); err != nil {
				return &ctrl.Result{}, errors.Wrapf(err, "failed to get IP address for VSphereMachine: %s", vsphereMachine.Name)
			}

			log.V(0).Info("waiting for IP address to be available for the VSphereMachine")
			return &ctrl.Result{}, nil
		}

		log.V(0).Info(fmt.Sprintf("static IP for %s is %s", vsphereMachine.Name, ipAddr.Name))
		if len(ipAddr.Spec.Address) == 0 {
			return &ctrl.Result{}, fmt.Errorf("failed to get IP address for the VSphereMachine: %s", vsphereMachine.Name)
		}

		ipSpec := ipAddr.Spec
		if ipSpec.Gateway == nil {
			return &ctrl.Result{}, errors.Wrapf(err, "invalid gateway assigned for IP address %s", ipAddr.Name)
		}

		ip := fmt.Sprintf("%s/%s", string(ipSpec.Address), strconv.Itoa(ipSpec.Prefix))
		log.V(0).Info(fmt.Sprintf("assigning IP address %s to VSphereMachine %s", ip, vsphereMachine.Name))
		dev.IPAddrs = []string{ip}
		gateway := string(*ipSpec.Gateway)
		//TODO: handle ipv6
		//gateway4 is required if DHCP4 is disabled, gateway6 is required if DHCP6 is disabled
		dev.Gateway4 = gateway
		dev.Nameservers = []string{"8.8.8.8"}

		updatedDevices = append(updatedDevices, dev)
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
