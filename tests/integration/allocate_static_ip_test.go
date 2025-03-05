package integration

import (
	. "github.com/metal3-io/ip-address-manager/controllers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/controllers"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	vSphereMachineReconciler *VSphereMachineReconciler
	vSphereClusterReconciler *VSphereClusterReconciler
	m3ipamReconciler         *IPPoolReconciler
	log                      = klog.Background().WithName("allocate-static-ip-test")
	key                      client.ObjectKey
	testClient               client.Client
	ctrlreq                  ctrl.Request
	ipamctrlreq              ctrl.Request
)

var _ = Describe("Static IP Allocation", func() {
	BeforeEach(func() {
		initVariables()
	})
	AfterEach(func() {})

	Context("Reconcile vSphere resources to allocate static IP", func() {
		It("should not error", func() {
			createPrerequisiteResources()
			verifyVSphereMachineStaticIPAllocation()
			verifyNameserversAndSearchDomainsAllocation()
			verifyVSphereClusterKubeVipAllocation()
		})
	})
})
