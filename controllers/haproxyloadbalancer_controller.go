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
	ctrl "sigs.k8s.io/controller-runtime"
)

// HAProxyLoadBalancerReconciler reconciles a HAProxyLoadBalancer object
type HAProxyLoadBalancerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=haproxyloadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=haproxyloadbalancers/status,verbs=get;update;patch
func (r *HAProxyLoadBalancerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("haproxyloadbalancer", req.NamespacedName)

	ctx := context.Background()
	log := r.Log
	var res *ctrl.Result
	var err error

	cluster := &capi.Cluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		return ctrl.Result{}, util.IgnoreNotFound(err)
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

func (r *HAProxyLoadBalancerReconciler) reconcileIP(cluster *capi.Cluster) (*ctrl.Result, error) {
	r.Log.V(0).Info("reconcile IP")

	//reconcile LB IP
	lbList := &infrav1.HAProxyLoadBalancerList{}
	if err := r.List(context.Background(), lbList, client.InNamespace(cluster.Namespace)); err != nil {
		return &ctrl.Result{}, util.IgnoreNotFound(err)
	}

	for _, item := range lbList.Items {
		if res, err := r.reconcileLoadBalancerIPAddress(cluster, &item); err != nil || res != nil {
			return res, err
		}
	}

	return nil, nil
}

func (r *HAProxyLoadBalancerReconciler) reconcileLoadBalancerIPAddress(cluster *capi.Cluster, lb *infrav1.HAProxyLoadBalancer) (*ctrl.Result, error) {
	log := r.Log

	//if DHCP is false, assign the static ip generated from the ip claim
	if lb != nil {
		log.V(0).Info(fmt.Sprintf("reconcile IP address for haproxy loadbalancer %s", lb.Name))
		devices := lb.Spec.VirtualMachineConfiguration.Network.Devices
		if len(devices) > 0 {
			//DHCP should be false on all devices, by default there is only one device
			if util.IsIPAllocationDHCP(devices) {
				log.V(0).Info("IP allocation is DHCP")
				return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			ipAddr, err := util.GetStaticIp(r.Client, cluster, lb.Name, r.Log)
			if err != nil {
				return nil, err
			}

			if ipAddr == nil {
				//if ip address list is not found, create a new ip claim
				if err := util.ReconcileIPClaim(r.Client, cluster, lb.Name, r.Log); err != nil {
					return nil, errors.Wrapf(err, "failed to get IP address for loadbalancer %s", lb.Name)
				}

				log.V(0).Info("waiting for IP address to be available for the loadbalancer, requeue the reconcile")
				return &ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			} else {
				log.V(0).Info(fmt.Sprintf("static IP for loadbalancer %s is %s", lb.Name, ipAddr.Name))
				if len(ipAddr.Spec.Address) > 0 {
					dataPatch := client.MergeFrom(lb.DeepCopy())
					ipSpec := ipAddr.Spec
					if ipSpec.Gateway == nil {
						return nil, fmt.Errorf("invalid gateway assigned for IP address %s", ipAddr.Name)
					}
					gateway := string(*ipSpec.Gateway)

					//if DHCP4 is disabled, gateway4 is required
					//if DHCP6 is disabled, gateway6 is required
					newDevices := []infrav1.NetworkDeviceSpec{}
					for _, dev := range lb.Spec.VirtualMachineConfiguration.Network.Devices {
						//TODO: handle ipv6
						if !dev.DHCP4 && len(dev.IPAddrs) == 0 {
							ip := fmt.Sprintf("%s/%s", string(ipSpec.Address), strconv.Itoa(ipSpec.Prefix))
							log.V(0).Info(fmt.Sprintf("assigning IP address %s to loadbalancer %s", ip, lb.Name))
							dev.IPAddrs = []string{ip}
							dev.Gateway4 = gateway
							dev.Nameservers = []string{"8.8.8.8"}
						}
						newDevices = append(newDevices, dev)
					}

					lb.Spec.VirtualMachineConfiguration.Network.Devices = newDevices
					if err := r.Patch(context.TODO(), lb.DeepCopyObject(), dataPatch); err != nil {
						return nil, errors.Wrapf(err, "failed to patch loadbalancer %s", lb.Name)
					} else {
						log.V(0).Info("loadbalancer patched successfully with static IP")
					}
				} else {
					return nil, fmt.Errorf("failed to get IP address for the loadbalancer: %s", lb.Name)
				}
				log.V(0).Info("reconcile loadbalancer IP address done")
			}
		} else {
			log.V(0).Info(fmt.Sprintf("no network device found for loadbalancer %s", lb.Name))
		}
	} else {
		return nil, fmt.Errorf("invalid loadbalancer: %s", lb.Name)
	}

	return nil, nil
}

func (r *HAProxyLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.HAProxyLoadBalancer{}).
		Complete(r)
}
