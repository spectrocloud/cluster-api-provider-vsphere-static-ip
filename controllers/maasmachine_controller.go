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
	infrav1 "github.com/spectrocloud/cluster-api-provider-maas/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MaasMachineReconciler reconciles a MaasMachine object
type MaasMachineReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=maasmachinetemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=maasmachines,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=maasmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ippools,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ippools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses/status,verbs=get;update;patch

func (r *MaasMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("maasmachine", req.NamespacedName)
	var res *ctrl.Result
	var err error

	maasMachine := &infrav1.MaasMachine{}
	if err := r.Get(ctx, req.NamespacedName, maasMachine); err != nil {
		return ctrl.Result{}, util.IgnoreNotFound(err)
	}

	//handle the case where gvk is empty
	if maasMachine.GroupVersionKind().Empty() {
		log.V(0).Info("setting the missing gvk for maasMachine")
		maasMachine.Kind = "MaasMachine"
		maasMachine.APIVersion = infrav1.GroupVersion.String()
	}

	// fetch the capi machine.
	machine, err := clusterutilv1.GetOwnerMachine(ctx, r.Client, maasMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.V(0).Info("waiting for machine controller to set ownerRef on MaasMachine")
		return ctrl.Result{}, nil
	}

	// fetch the capi cluster
	cluster, err := clusterutilv1.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.V(0).Info("machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	res, err = r.reconcileMaasMachineIPAddress(cluster, maasMachine)
	if err != nil {
		log.Error(err, "failed to reconcile MaasMachine IP")
	}

	if res == nil {
		res = &ctrl.Result{}
	}

	return *res, err
}

func (r *MaasMachineReconciler) reconcileMaasMachineIPAddress(cluster *capi.Cluster, maasMachine *infrav1.MaasMachine) (*ctrl.Result, error) {
	if maasMachine == nil {
		r.Log.V(0).Info("invalid MaasMachine, skipping reconcile IPAddress")
		return &ctrl.Result{}, nil
	}

	log := r.Log.WithValues("maasMachine", maasMachine.Name, "namespace", maasMachine.Namespace)
	log.V(0).Info("reconcile IP address for MaasMachine")

	// Check if machine has ipAllocationType field and if it's set to static
	// if maasMachine.Spec.IpAllocationType == nil || *maasMachine.Spec.IpAllocationType != "static" {
	// 	log.V(0).Info("MaasMachine has allocation type other than static, skipping IP allocation")
	// 	return &ctrl.Result{}, nil
	// }

	// // Check if IP address is already allocated
	// if maasMachine.Spec.IpAddress != nil && len(*maasMachine.Spec.IpAddress) > 0 {
	// 	log.V(0).Info("IP address is already allocated for MaasMachine", "ipAddress", *maasMachine.Spec.IpAddress)
	// 	return &ctrl.Result{}, nil
	// }

	dataPatch := client.MergeFrom(maasMachine.DeepCopy())
	newIpamFunc, ok := factory.IpamFactory[ipam.IpamTypeMetal3io]
	if !ok {
		log.V(0).Info("ipam type not supported")
		return &ctrl.Result{}, nil
	}

	ipamFunc := newIpamFunc(r.Client, log)

	poolMatchLabels, err := r.getIPPoolMatchLabels(r.Client, maasMachine)
	if err != nil {
		log.Error(err, "failed to get IPPool match labels")
		return &ctrl.Result{}, nil
	}
	ipPool, err := ipamFunc.GetAvailableIPPool(poolMatchLabels, cluster.ObjectMeta)
	if err != nil {
		log.Error(err, "failed to get an available IPPool")
		return &ctrl.Result{}, nil
	}
	if ipPool == nil {
		log.V(0).Info("waiting for IPPool to be available")
		return &ctrl.Result{}, nil
	}

	ipName := maasMachine.Name
	ip, err := ipamFunc.GetIP(ipName, ipPool)
	if err != nil {
		return &ctrl.Result{}, errors.Wrapf(err, "failed to get allocated IP address for MaasMachine %s", maasMachine.Name)
	}

	if ip == nil {
		if _, err := ipamFunc.AllocateIP(ipName, ipPool, maasMachine); err != nil {
			return &ctrl.Result{}, errors.Wrapf(err, "failed to allocate IP address for MaasMachine: %s", maasMachine.Name)
		}

		log.V(0).Info("waiting for IP address to be available for the MaasMachine")
		return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err := util.ValidateIP(ip); err != nil {
		return &ctrl.Result{}, errors.Wrapf(err, "invalid IP address retrieved for MaasMachine: %s", maasMachine.Name)
	}

	log.V(0).Info("static IP selected for MaasMachine", "IPAddressName", ip.GetName())

	ipAddr := util.GetAddress(ip)
	log.V(0).Info("assigning IP address to MaasMachine", "IPAddress", ipAddr)

	// Set the IP address in the MaasMachine spec
	// maasMachine.Spec.IpAddress = &ipAddr

	if err := r.Patch(context.TODO(), maasMachine, dataPatch); err != nil {
		return &ctrl.Result{}, errors.Wrapf(err, "failed to patch MaasMachine %s", maasMachine.Name)
	}

	log.V(0).Info("successfully reconciled IP address for MaasMachine")

	return &ctrl.Result{}, nil
}

func (r *MaasMachineReconciler) getIPPoolMatchLabels(cli client.Client, maasMachine *infrav1.MaasMachine) (map[string]string, error) {

	//match labels for the IPPool are retrieved from the MaasMachineTemplate
	vmTemplateName, ok := maasMachine.GetAnnotations()[capi.TemplateClonedFromNameAnnotation]
	if !ok {
		return nil, fmt.Errorf("MaasMachine %s has no value set in the 'cloned-from-name' annotation", maasMachine.Name)
	}

	maasMachineTemplate := &infrav1.MaasMachineTemplate{}
	key := types.NamespacedName{Namespace: maasMachine.Namespace, Name: vmTemplateName}
	if err := cli.Get(context.Background(), key, maasMachineTemplate); err != nil {
		return nil, fmt.Errorf("failed to get MaasMachineTemplate %s", vmTemplateName)
	}

	return maasMachineTemplate.GetLabels(), nil
}

func (r *MaasMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.MaasMachine{}).
		Complete(r)
}
