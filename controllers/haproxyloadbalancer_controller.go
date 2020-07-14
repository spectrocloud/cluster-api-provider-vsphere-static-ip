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
	numOfDevices := len(devices)
	log.V(0).Info(fmt.Sprintf("reconcile IP address for HAProxyLoadBalancer %s, with a device count of %d", lb.Name, numOfDevices))
	if numOfDevices == 0 {
		log.V(0).Info(fmt.Sprintf("no network device found for HAProxyLoadBalancer %s", lb.Name))
		return &ctrl.Result{}, nil
	}

	updatedDevices := []infrav1.NetworkDeviceSpec{}
	dataPatch := client.MergeFrom(lb.DeepCopy())

	for _, dev := range devices {
		if util.IsIPAllocationDHCP(dev) || len(dev.IPAddrs) > 0 {
			updatedDevices = append(updatedDevices, dev)
			continue
		}

		ipAddr, err := util.GetStaticIp(r.Client, cluster, lb.Name, r.Log)
		if err != nil {
			return &ctrl.Result{}, err
		}

		if ipAddr == nil {
			//if ip address list is not found, create a new ip claim
			if err := util.ReconcileIPClaim(r.Client, cluster, lb, r.Log); err != nil {
				return nil, errors.Wrapf(err, "failed to get IP address for HAProxyLoadBalancer %s", lb.Name)
			}

			log.V(0).Info("waiting for IP address to be available for the HAProxyLoadBalancer")
			return &ctrl.Result{}, nil
		}

		log.V(0).Info(fmt.Sprintf("static IP for %s is %s", lb.Name, ipAddr.Name))
		if len(ipAddr.Spec.Address) == 0 {
			return &ctrl.Result{}, fmt.Errorf("failed to get IP address for the HAProxyLoadBalancer: %s", lb.Name)
		}

		ipSpec := ipAddr.Spec
		if ipSpec.Gateway == nil {
			return &ctrl.Result{}, errors.Wrapf(err, "invalid gateway assigned for IP address %s", ipAddr.Name)
		}

		ip := fmt.Sprintf("%s/%s", string(ipSpec.Address), strconv.Itoa(ipSpec.Prefix))
		log.V(0).Info(fmt.Sprintf("assigning IP address %s to HAProxyLoadBalancer %s", ip, lb.Name))
		dev.IPAddrs = []string{ip}
		gateway := string(*ipSpec.Gateway)
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
	return &ctrl.Result{}, nil
}

func (r *HAProxyLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.HAProxyLoadBalancer{}).
		Complete(r)
}
