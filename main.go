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
	"os"
	"time"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/controllers"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	kubeadmcontrolplane "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/util/flags"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

var (
	scheme         = runtime.NewScheme()
	setupLog       = ctrl.Log.WithName("setup")
	managerOptions = flags.ManagerOptions{}
	logOptions     = logs.NewOptions()
	//flags
	watchNamespace          string
	syncPeriod              time.Duration
	metricsAddr             string
	enableLeaderElection    bool
	maxConcurrentReconciles int
	webhookPort             int
	webhookCertDir          string
	webhookCertName         string
	webhookKeyName          string
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

func InitFlags(fs *pflag.FlagSet) {
	logsv1.AddFlags(logOptions, fs)
	fs.DurationVar(&syncPeriod, "sync-period", 10*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 5m)")

	fs.StringVar(&metricsAddr, "metrics-addr", ":8080",
		"The address the metric endpoint binds to.")

	fs.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	fs.IntVar(&maxConcurrentReconciles, "max-concurrency", 2,
		"MaxConcurrentReconciles is the maximum number of concurrent Reconciles which can be run")

	fs.IntVar(&webhookPort, "webhook-port", 9443,
		"Webhook Server port")

	fs.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir.")

	fs.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt",
		"Webhook cert name.")

	fs.StringVar(&webhookKeyName, "webhook-key-name", "tls.key",
		"Webhook key name.")

	flags.AddManagerOptions(fs, &managerOptions)
	feature.MutableGates.AddFlag(fs)
}

func main() {
	InitFlags(pflag.CommandLine)

	//ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	ctrl.SetLogger(klogr.New())
	// if watchNamespace == "" {
	// 	setupLog.Info("namespace is not specified. will watch over all namespaces")
	// } else {
	// 	setupLog.Info("watch in a single namespace", "namespace", watchNamespace)
	// }
	setupLog.Info("resync period", "every", syncPeriod)

	var watchNamespaces map[string]cache.Config
	if watchNamespace != "" {
		watchNamespaces = map[string]cache.Config{
			watchNamespace: {},
		}
	}

	tlsOptions, metricsOptions, err := flags.GetManagerOptions(managerOptions)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		// MetricsBindAddress: metricsAddr,
		Metrics:          *metricsOptions,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "controller-leader-election-capv-static-ip",
		// SyncPeriod:         &syncPeriod,
		// Namespace:          watchNamespace,
		Cache: cache.Options{
			DefaultNamespaces: watchNamespaces,
			SyncPeriod:        &syncPeriod,
		},
		WebhookServer: webhook.NewServer(
			webhook.Options{
				Port:     webhookPort,
				CertDir:  webhookCertDir,
				CertName: webhookCertName,
				KeyName:  webhookKeyName,
				TLSOpts:  tlsOptions,
			},
		),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
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
