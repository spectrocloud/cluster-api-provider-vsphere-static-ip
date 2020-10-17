package integration

import (
	"context"
	"flag"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/tests/integration/manager"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

var (
	tm  *manager.Manager
	ctx = context.Background()
)

func TestStaticIPAllocation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Integration Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	klog.InitFlags(nil)
	flag.CommandLine.Set("v", "2")
	flag.Parse()
	ctrl.SetLogger(klogr.New())

	By("bootstrapping test environment")

	tm = manager.NewTestManager()
	tm.LoadTestEnv()
	tm.InitEnvironment(manager.InitEnvironmentInput{
		Name: "test",
		CRDs: []string{
			manager.CapiCRD,
			manager.CapvCRD,
			manager.M3IpamCRD,
		},
	})
	tm.SaveKubeconfig("/tmp/kubeconfig-current")
	err := os.Setenv("KUBECONFIG", "/tmp/kubeconfig-current")
	Expect(err).To(Not(HaveOccurred()))
	close(done)
}, 120)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	tm.DestroyEnvironment()
})
