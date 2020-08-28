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

	"github.com/pkg/errors"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam/factory"

	"github.com/go-logr/logr"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
)

// VSphereClusterReconciler reconciles a VSphereCluster object
type VSphereClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters/status,verbs=get;update;patch

func (r *VSphereClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("vspherecluster", req.NamespacedName)
	var res *ctrl.Result
	var err error

	cluster := &capi.Cluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		return ctrl.Result{}, util.IgnoreNotFound(err)
	}

	vSphereCluster := &infrav1.VSphereCluster{}
	if err := r.Get(ctx, req.NamespacedName, vSphereCluster); err != nil {
		return ctrl.Result{}, util.IgnoreNotFound(err)
	}

	res, err = r.reconcileVSphereClusterControlPlaneEndpoint(cluster, vSphereCluster)
	if err != nil {
		log.Error(err, "failed to reconcile VSphereCluster control plane endpoint")
	}

	if res == nil {
		res = &ctrl.Result{}
	}

	return *res, err
}

func (r *VSphereClusterReconciler) reconcileVSphereClusterControlPlaneEndpoint(cluster *capi.Cluster, vSphereCluster *infrav1.VSphereCluster) (*ctrl.Result, error) {
	if vSphereCluster == nil {
		r.Log.V(0).Info("invalid VSphereCluster, skipping reconcile control plane endpoint")
		return &ctrl.Result{}, nil
	}

	log := r.Log.WithValues("vsphereCluster", vSphereCluster.Name, "namespace", vSphereCluster.Namespace)
	log.V(0).Info("reconcile control plane endpoint address for VSphereCluster")

	if vSphereCluster.Spec.LoadBalancerRef != nil {
		log.V(0).Info("VSphereCluster is using HAProxyLoadbalancer, skipping reconcile for control plane endpoint")
		return &ctrl.Result{}, nil
	}

	if len(vSphereCluster.Spec.ControlPlaneEndpoint.Host) > 0 {
		log.V(0).Info("control plane endpoint is already allocated for the VSphereCluster", "vSphereCluster", vSphereCluster.Name)
		return &ctrl.Result{}, nil
	}

	newIpamFunc, ok := factory.IpamFactory[ipam.IpamTypeMetal3io]
	if !ok {
		log.V(0).Info("ipam type not supported")
		return &ctrl.Result{}, nil
	}

	dataPatch := client.MergeFrom(vSphereCluster.DeepCopy())
	ipamFunc := newIpamFunc(r.Client, log)

	ipPool, err := ipamFunc.GetAvailableIPPool(vSphereCluster.Labels, cluster.ObjectMeta)
	if err != nil {
		log.Error(err, "failed to get an available IPPool")
		return &ctrl.Result{}, nil
	}
	if ipPool == nil {
		log.V(0).Info("waiting for IPPool to be available")
		return &ctrl.Result{}, nil
	}

	ipName := vSphereCluster.Name
	ip, err := ipamFunc.GetIP(ipName, ipPool)
	if err != nil {
		return &ctrl.Result{}, errors.Wrapf(err, "failed to get allocated IP address for VSphereCluster %s", vSphereCluster.Name)
	}

	if ip == nil {
		if _, err := ipamFunc.AllocateIP(ipName, ipPool, vSphereCluster); err != nil {
			return &ctrl.Result{}, errors.Wrapf(err, "failed to allocate IP address for VSphereCluster %s", vSphereCluster.Name)
		}

		log.V(0).Info("waiting for IP address to be available for the VSphereCluster")
		return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err := util.ValidateIP(ip); err != nil {
		return &ctrl.Result{}, errors.Wrapf(err, "invalid IP address retrieved for VSphereCluster: %s", vSphereCluster.Name)
	}

	ipAddr := util.GetAddress(ip)
	log.V(0).Info(fmt.Sprintf("allocating control plane endpoint %s for VSphereCluster %s", ipAddr, vSphereCluster.Name))

	vSphereCluster.Spec.ControlPlaneEndpoint.Host = ipAddr
	if err := r.Patch(context.TODO(), vSphereCluster.DeepCopyObject(), dataPatch); err != nil {
		return &ctrl.Result{}, errors.Wrapf(err, "failed to patch VSphereCluster %s", vSphereCluster.Name)
	}

	log.V(0).Info("successfully reconciled control plane endpoint for VSphereCluster")

	return &ctrl.Result{}, nil
}

func (r *VSphereClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.VSphereCluster{}).
		Complete(r)
}
