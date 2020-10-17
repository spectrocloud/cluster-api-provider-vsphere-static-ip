package integration

import (
	. "github.com/metal3-io/ip-address-manager/controllers"
	. "github.com/onsi/ginkgo"
	. "github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/controllers"
	"k8s.io/klog/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	vSphereMachineReconciler *VSphereMachineReconciler
	vSphereClusterReconciler *VSphereClusterReconciler
	m3ipamReconciler         *IPPoolReconciler
	log                      = klogr.New().WithName("allocate-static-ip-test")
	key                      client.ObjectKey
	ctrlreq                  ctrl.Request
	ipamctrlreq              ctrl.Request
)

var _ = Describe("VSphereMachine Controller", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile vSphere machines", func() {
		It("should not error", func() {
			initVariables()
			createPrerequisiteResources()
			verifyVSphereMachineStaticIPAllocation()
			verifyVSphereClusterKubeVipAllocation()
		})
	})
})
