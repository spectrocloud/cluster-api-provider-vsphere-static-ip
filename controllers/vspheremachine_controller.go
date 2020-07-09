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
	"time"

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

	res, err = r.reconcileIP(cluster)
	if err != nil {
		log.Error(err, "failed to reconcile IP")
	}

	if res == nil {
		res = &ctrl.Result{}
	}

	return *res, err
}

func (r *VSphereMachineReconciler) reconcileIP(cluster *capi.Cluster) (*ctrl.Result, error) {
	r.Log.V(0).Info("reconcile IP")

	//reconcile vsphere machine ip
	vmList := &infrav1.VSphereMachineList{}
	if err := r.List(context.Background(), vmList, client.InNamespace(cluster.Namespace)); err != nil {
		return &ctrl.Result{}, util.IgnoreNotFound(err)
	}

	for _, item := range vmList.Items {
		if res, err := r.reconcileVSphereMachineIPAddress(cluster, &item); err != nil || res != nil {
			return res, err
		}
	}

	return nil, nil
}

func (r *VSphereMachineReconciler) reconcileVSphereMachineIPAddress(cluster *capi.Cluster, vsphereMachine *infrav1.VSphereMachine) (*ctrl.Result, error) {
	log := r.Log

	if vsphereMachine != nil {
		log.V(0).Info(fmt.Sprintf("reconcile ip address for vsphere machine %s", vsphereMachine.Name))
		devices := vsphereMachine.Spec.VirtualMachineCloneSpec.Network.Devices
		log.V(0).Info(fmt.Sprintf("Number of devices for %s is %d", vsphereMachine.Name, len(devices)))

		if len(devices) > 0 {
			if util.IsIPAllocationDHCP(devices) {
				log.V(0).Info("IP allocation is DHCP")
				return &ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
			}

			ipAddr, err := util.GetStaticIp(r.Client, cluster, vsphereMachine.Name, r.Log)
			if err != nil {
				return nil, err
			}

			if ipAddr == nil {
				//if ip address list is not found, create a new ip claim
				if err := util.ReconcileIPClaim(r.Client, cluster, vsphereMachine, r.Log); err != nil {
					return nil, errors.Wrapf(err, "failed to get IP address for vsphere machine: %s", vsphereMachine.Name)
				}

				log.V(0).Info("waiting for IP address to be available for the vsphere machine, requeue the reconcile")
				return &ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			} else {
				log.V(0).Info(fmt.Sprintf("static IP for %s is %s", vsphereMachine.Name, ipAddr.Name))
				if len(ipAddr.Spec.Address) > 0 {
					dataPatch := client.MergeFrom(vsphereMachine.DeepCopy())
					ipSpec := ipAddr.Spec
					if ipSpec.Gateway == nil {
						return nil, errors.Wrapf(err, "invalid gateway assigned for IP address %s", ipAddr.Name)
					}
					gateway := string(*ipSpec.Gateway)

					//if DHCP4 is disabled, gateway4 is required
					//if DHCP6 is disabled, gateway6 is required
					newDevices := []infrav1.NetworkDeviceSpec{}
					for _, dev := range vsphereMachine.Spec.VirtualMachineCloneSpec.Network.Devices {
						//TODO: handle ipv6
						if !dev.DHCP4 && len(dev.IPAddrs) <= 0 {
							ip := fmt.Sprintf("%s/%s", string(ipSpec.Address), strconv.Itoa(ipSpec.Prefix))
							log.V(0).Info(fmt.Sprintf("assigning ip address %s to vsphere machine %s", ip, vsphereMachine.Name))
							dev.IPAddrs = []string{ip}
							dev.Gateway4 = gateway
							dev.Nameservers = []string{"8.8.8.8"}
						}
						newDevices = append(newDevices, dev)
					}
					vsphereMachine.Spec.VirtualMachineCloneSpec.Network.Devices = newDevices

					if err := r.Patch(context.TODO(), vsphereMachine.DeepCopyObject(), dataPatch); err != nil {
						return nil, errors.Wrapf(err, "failed to patch vsphere machine %s", vsphereMachine.Name)
					} else {
						log.V(0).Info("vsphere machine patched successfully with static IP")
					}
				} else {
					return nil, fmt.Errorf("failed to get IP address for the vsphere machine: %s", vsphereMachine.Name)
				}
				log.V(0).Info("reconcile vsphere machine IP address done")
			}
		} else {
			log.V(0).Info(fmt.Sprintf("no network device found for vsphere machine %s", vsphereMachine.Name))
		}
	} else {
		return nil, fmt.Errorf("invalid vsphere machine: %s", vsphereMachine.Name)
	}

	return nil, nil
}

func (r *VSphereMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.VSphereMachine{}).
		Complete(r)
}
