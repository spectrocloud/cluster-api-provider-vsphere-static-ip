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

package main

import (
	"flag"
	"os"
	"time"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	kubeadmcontrolplane "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = ipamv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = capi.AddToScheme(scheme)
	_ = capi.AddToScheme(scheme)
	_ = kubeadmcontrolplane.AddToScheme(scheme)

	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		watchNamespace          string
		syncPeriod              time.Duration
		metricsAddr             string
		enableLeaderElection    bool
		maxConcurrentReconciles int
	)

	flag.StringVar(&watchNamespace, "namespace", "", "Namespace that the controller watches. If not specified, will watch over all namespaces.")
	flag.DurationVar(&syncPeriod, "sync-period", 10*time.Minute, "The minimum interval at which watched resources are reconciled (e.g. 5m)")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&maxConcurrentReconciles, "max-concurrency", 2, "MaxConcurrentReconciles is the maximum number of concurrent Reconciles which can be run")
	flag.Parse()

	//ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	ctrl.SetLogger(klogr.New())
	if watchNamespace == "" {
		setupLog.Info("namespace is not specified. will watch over all namespaces")
	} else {
		setupLog.Info("watch in a single namespace", "namespace", watchNamespace)
	}
	setupLog.Info("resync period", "every", syncPeriod)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "controller-leader-election-capv-static-ip",
		SyncPeriod:         &syncPeriod,
		Namespace:          watchNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.HAProxyLoadBalancerReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("HAProxyLoadBalancer"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HAProxyLoadBalancer")
		os.Exit(1)
	}
	if err = (&controllers.VSphereMachineReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("VSphereMachine"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VSphereMachine")
		os.Exit(1)
	}
	if err = (&controllers.VSphereClusterReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("VSphereCluster"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VSphereCluster")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
