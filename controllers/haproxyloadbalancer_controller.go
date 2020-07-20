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

	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam/factory"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

	haProxyLB := &infrav1.HAProxyLoadBalancer{}
	if err := r.Get(ctx, req.NamespacedName, haProxyLB); err != nil {
		return ctrl.Result{}, util.IgnoreNotFound(err)
	}

	res, err = r.reconcileLoadBalancerIPAddress(cluster, haProxyLB)
	if err != nil {
		log.Error(err, "failed to reconcile HAProxyLoadbalancer IP")
	}

	if res == nil {
		res = &ctrl.Result{}
	}

	return *res, err
}

func (r *HAProxyLoadBalancerReconciler) reconcileLoadBalancerIPAddress(cluster *capi.Cluster, lb *infrav1.HAProxyLoadBalancer) (*ctrl.Result, error) {
	log := r.Log

	if lb == nil {
		return nil, fmt.Errorf("invalid HAProxyLoadBalancer: %s", lb.Name)
	}

	devices := lb.Spec.VirtualMachineConfiguration.Network.Devices
	log.V(0).Info(fmt.Sprintf("reconcile IP address for HAProxyLoadBalancer %s", lb.Name))
	if len(devices) == 0 {
		log.V(0).Info(fmt.Sprintf("no network device found for HAProxyLoadBalancer %s", lb.Name))
		return &ctrl.Result{}, nil
	}

	if util.IsMachineIPAllocationDHCP(devices) {
		log.V(0).Info(fmt.Sprintf("HAProxyLoadBalancer %s has allocation type DHCP", lb.Name))
		return &ctrl.Result{}, nil
	}

	updatedDevices := []infrav1.NetworkDeviceSpec{}
	dataPatch := client.MergeFrom(lb.DeepCopy())

	if newIpamFunc, ok := factory.IpamFactory[ipam.IpamTypeMetal3io]; ok {
		f := newIpamFunc(r.Client, log)

		for _, dev := range devices {
			if util.IsDeviceIPAllocationDHCP(dev) || len(dev.IPAddrs) > 0 {
				updatedDevices = append(updatedDevices, dev)
				continue
			}

			key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name}
			ip, err := f.GetIP("", key, lb)
			if err != nil {
				return &ctrl.Result{}, err
			}

			if ip == nil {
				//generate a new static IP for the resource
				if err := f.AllocateIP("", key, lb); err != nil {
					return nil, errors.Wrapf(err, "failed to get IP address for HAProxyLoadBalancer %s", lb.Name)
				}

				log.V(0).Info("waiting for IP address to be available for the HAProxyLoadBalancer")
				return &ctrl.Result{}, nil
			}

			if err := util.ValidateIP(ip); err != nil {
				return &ctrl.Result{}, errors.Wrapf(err, "invalid IP address retrieved for HAProxyLoadBalancer: %s", lb.Name)
			}

			log.V(0).Info(fmt.Sprintf("static IP for %s is %s", lb.Name, ip.GetName()))

			//capv expects static-ip in the CIDR format
			ipCidr := fmt.Sprintf("%s/%d", util.GetAddress(ip), util.GetMask(ip))
			log.V(0).Info(fmt.Sprintf("assigning IP address %s to HAProxyLoadBalancer %s", util.GetAddress(ip), lb.Name))
			dev.IPAddrs = []string{ipCidr}
			gateway := util.GetGateway(ip)
			//TODO: handle ipv6
			//gateway4 is required if DHCP4 is disabled, gateway6 is required if DHCP6 is disabled
			dev.Gateway4 = gateway
			dev.Nameservers = []string{"8.8.8.8"}

			updatedDevices = append(updatedDevices, dev)
		}

		lb.Spec.VirtualMachineConfiguration.Network.Devices = updatedDevices
		if err := r.Patch(context.TODO(), lb.DeepCopyObject(), dataPatch); err != nil {
			return &ctrl.Result{}, errors.Wrapf(err, "failed to patch HAProxyLoadBalancer %s", lb.Name)
		}

		log.V(0).Info("successfully reconciled IP address for HAProxyLoadBalancer")
	} else {
		return &ctrl.Result{}, fmt.Errorf("ipam type not supported")
	}

	return &ctrl.Result{}, nil
}

func (r *HAProxyLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.HAProxyLoadBalancer{}).
		Complete(r)
}
