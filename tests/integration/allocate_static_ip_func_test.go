package integration

import (
	"os"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/metal3-io/ip-address-manager/controllers"
	"github.com/metal3-io/ip-address-manager/ipam"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/controllers"
	. "github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capivsphere "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha4"
	kubeadmv3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha4"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	LabelControlPlane = "cluster.x-k8s.io/control-plane"
	LabelIPPoolName   = "cluster.x-k8s.io/ip-pool-name"
)

func logInfoLine(info string) {
	log.V(0).Info("########################################################", "info", info)
}

func initVariables() {
	err := os.Setenv("KUBECONFIG", "/tmp/kubeconfig-current")
	Expect(err).To(Not(HaveOccurred()))

	vSphereMachineReconciler = &VSphereMachineReconciler{
		Client: tm.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("VSphereMachine"),
	}

	vSphereClusterReconciler = &VSphereClusterReconciler{
		Client: tm.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("VSphereCluster"),
	}

	objects := []runtime.Object{}
	objects = append(objects, tm.M3IpamIPPool)
	objects = append(objects, tm.Cluster)
	objects = append(objects, tm.Machine)
	objects = append(objects, tm.KubeadmControlPlane)

	ipamClient := fake.NewFakeClientWithScheme(setupScheme(), objects...)
	m3ipamReconciler = &IPPoolReconciler{
		Client:         ipamClient,
		Log:            ctrl.Log.WithName("controllers").WithName("IPPool"),
		ManagerFactory: ipam.NewManagerFactory(tm.GetClient()),
	}

	key = client.ObjectKey{
		Namespace: tm.Cluster.Namespace,
		Name:      tm.Cluster.Name,
	}

	ctrlreq = ctrl.Request{
		NamespacedName: key,
	}

	ipamctrlreq = ctrl.Request{
		NamespacedName: client.ObjectKey{
			Namespace: tm.Cluster.Namespace,
			Name:      "ip-pool-pool1",
		},
	}
}

func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	err := capiv1alpha3.AddToScheme(s)
	Expect(err).NotTo(HaveOccurred())
	err = capivsphere.AddToScheme(s)
	Expect(err).NotTo(HaveOccurred())
	err = ipamv1.AddToScheme(s)
	Expect(err).ToNot(HaveOccurred())
	err = kubeadmv3.AddToScheme(s)
	Expect(err).ToNot(HaveOccurred())

	return s
}

func createPrerequisiteResources() {
	logInfoLine("createPrerequisiteResources")

	By("creation of m3ippool should succeed")
	tmippool := tm.M3IpamIPPool.DeepCopy()
	tmippool.SetResourceVersion("")
	Expect(tm.GetClient().Create(ctx, tmippool)).To(Succeed())

	By("creation of capi cluster should succeed")
	tmcluster := tm.Cluster.DeepCopy()
	tmcluster.SetResourceVersion("")
	Expect(tm.GetClient().Create(ctx, tmcluster)).To(Succeed())
}

func verifyVSphereMachineStaticIPAllocation() {
	logInfoLine("verifyVSphereMachineStaticIPAllocation")

	By("creation of machine should succeed")
	tmMachine := tm.Machine.DeepCopy()
	tmMachine.SetResourceVersion("")
	Expect(tm.GetClient().Create(ctx, tmMachine)).To(Succeed())

	By("creation of VSphereMachine with DHCP set to true should succeed")
	tmvpshereMachine := tm.VSphereMachine.DeepCopy()
	tmvpshereMachine.SetResourceVersion("")
	Expect(tm.GetClient().Create(ctx, tmvpshereMachine)).To(Succeed())

	By("reconcile of VSphereMachine with DHCP should skip static IP Allocation")
	testVSphereMachineReconcileSuccess(getReconcileRequest(tm.VSphereMachine.Name, tm.VSphereMachine.Namespace))

	By("ipam reconcile without any IPClaim should skip creation of IPAddress")
	result, err := m3ipamReconciler.Reconcile(ctx, ipamctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())
	ipList := &ipamv1.IPAddressList{}
	Expect(tm.GetClient().List(ctx, ipList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(err).To(BeNil())
	Expect(len(ipList.Items)).To(Equal(0))

	cpTemplateName := "control-plane-template"

	By("creation of control-plane VSphereMachineTemplate with ip-pool-name label should succeed")
	cpVSphereMachineTemp := tm.VSphereMachineTemplate.DeepCopy()
	cpVSphereMachineTemp.Name = cpTemplateName
	cpVSphereMachineTemp.SetLabels(map[string]string{LabelIPPoolName: "ip-pool-pool1"})
	Expect(tm.GetClient().Create(ctx, cpVSphereMachineTemp)).To(Succeed())

	By("creation of kubeadm control plane should succeed")
	kcp := tm.KubeadmControlPlane.DeepCopy()
	kcp.SetResourceVersion("")
	kcp.Spec.MachineTemplate.InfrastructureRef.Name = cpTemplateName
	Expect(tm.GetClient().Create(ctx, kcp)).To(Succeed())

	By("creation of control-plane VSphereMachine with DHCP set to false should succeed")
	vSphereMachineName := "cp-vsphere-machine-1"
	createNewVSphereMachine(vSphereMachineName, true, cpVSphereMachineTemp)

	By("first VSphereMachine reconcile should create IPClaim and wait for IPAddress")
	testVSphereMachineReconcileRequeue(getReconcileRequest(vSphereMachineName, tm.VSphereMachine.Namespace))
	ipClaimList := &ipamv1.IPClaimList{}
	Expect(tm.GetClient().List(ctx, ipClaimList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(len(ipClaimList.Items)).To(Equal(1))

	By("ipam reconcile should create an IPAddress for the existing IPClaim")
	result, err = m3ipamReconciler.Reconcile(ctx, ipamctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())
	ipAddressList := &ipamv1.IPAddressList{}
	Expect(tm.GetClient().List(ctx, ipAddressList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(err).To(BeNil())
	Expect(len(ipAddressList.Items)).To(Equal(1))
	Expect(string(ipAddressList.Items[0].Spec.Address)).To(Equal("10.10.100.20"))

	By("second VSphereMachine reconcile should allocate the IPAddress to the waiting VSphereMachine")
	testVSphereMachineReconcileSuccess(getReconcileRequest(vSphereMachineName, tm.VSphereMachine.Namespace))

	updatedCPVSphereMachine := &infrav1.VSphereMachine{}
	Expect(tm.GetClient().Get(ctx, client.ObjectKey{
		Namespace: tm.VSphereMachine.Namespace,
		Name:      vSphereMachineName,
	}, updatedCPVSphereMachine)).To(Succeed())
	updatedCPVSphereMachineNw := updatedCPVSphereMachine.Spec.Network
	Expect(updatedCPVSphereMachineNw.Devices[0].IPAddrs[0]).To(Equal("10.10.100.20/18"))
}

func verifyNameserversAndSearchDomainsAllocation() {
	logInfoLine("verifyNameserversAndSearchDomainsAllocation")

	By("VSphereMachine's network device should not be configured with nameservers and searchDomains, " +
		"if these values are not configured in the IPPool and VSphereMachineTemplate")
	vSphereMachineName := "cp-vsphere-machine-1"
	existingCPVSphereMachine := &infrav1.VSphereMachine{}
	Expect(tm.GetClient().Get(ctx, client.ObjectKey{
		Namespace: tm.VSphereMachine.Namespace,
		Name:      vSphereMachineName,
	}, existingCPVSphereMachine)).To(Succeed())
	Expect(existingCPVSphereMachine.Spec.Network.Devices[0].Nameservers).To(BeEmpty())
	Expect(existingCPVSphereMachine.Spec.Network.Devices[0].SearchDomains).To(BeEmpty())

	By("VSphereMachine reconcile with nameservers and searchDomains not configured in IPPool, but configured" +
		" only in the cp VSphereMachineTemplate, should configure the values in the cp VSphereMachine's network device")
	vSphereMachineTmpl := &infrav1.VSphereMachineTemplate{}
	Expect(tm.GetClient().Get(ctx, client.ObjectKey{
		Namespace: tm.VSphereMachineTemplate.Namespace,
		Name:      "control-plane-template",
	}, vSphereMachineTmpl)).To(Succeed())
	vSphereMachineTmpl.Spec.Template.Spec.Network.Devices[0].Nameservers = []string{"1.2.3.4"}
	vSphereMachineTmpl.Spec.Template.Spec.Network.Devices[0].SearchDomains = []string{"company.com"}
	Expect(tm.GetClient().Update(ctx, vSphereMachineTmpl)).To(Succeed())

	vSphereMachineName = "cp-vsphere-machine-2"
	createNewVSphereMachine(vSphereMachineName, true, vSphereMachineTmpl)

	By("first VSphereMachine reconcile should create IPClaim and wait for IPAddress")
	testVSphereMachineReconcileRequeue(getReconcileRequest(vSphereMachineName, tm.VSphereMachine.Namespace))
	ipClaimList := &ipamv1.IPClaimList{}
	Expect(tm.GetClient().List(ctx, ipClaimList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(len(ipClaimList.Items)).To(Equal(2))

	By("ipam reconcile should create an IPAddress for the existing IPClaim")
	result, err := m3ipamReconciler.Reconcile(ctx, ipamctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())
	ipAddressList := &ipamv1.IPAddressList{}
	Expect(tm.GetClient().List(ctx, ipAddressList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(err).To(BeNil())
	Expect(len(ipAddressList.Items)).To(Equal(2))

	By("second VSphereMachine reconcile should allocate the IPAddress to the waiting VSphereMachine")
	testVSphereMachineReconcileSuccess(getReconcileRequest(vSphereMachineName, tm.VSphereMachine.Namespace))

	updatedCPVSphereMachine := &infrav1.VSphereMachine{}
	Expect(tm.GetClient().Get(ctx, client.ObjectKey{
		Namespace: tm.VSphereMachine.Namespace,
		Name:      vSphereMachineName,
	}, updatedCPVSphereMachine)).To(Succeed())
	Expect(updatedCPVSphereMachine.Spec.Network.Devices[0].IPAddrs[0]).To(Equal("10.10.100.21/18"))
	Expect(updatedCPVSphereMachine.Spec.Network.Devices[0].Nameservers[0]).To(Equal("1.2.3.4"))
	Expect(updatedCPVSphereMachine.Spec.Network.Devices[0].SearchDomains[0]).To(Equal("company.com"))

	By("resetting VSphereMachine's ipaddress should succeed")
	updatedCPVSphereMachine.Spec.Network.Devices[0].IPAddrs = []string{}
	Expect(tm.GetClient().Update(ctx, updatedCPVSphereMachine)).To(Succeed())

	By("updating the IPPool with nameservers and searchDomains should succeed")
	existingIPool := &ipamv1.IPPool{}
	Expect(tm.GetClient().Get(ctx, client.ObjectKey{
		Namespace: tm.M3IpamIPPool.Namespace,
		Name:      tm.M3IpamIPPool.Name,
	}, existingIPool)).To(Succeed())

	//updating nameservers and searchDomains
	existingIPool.Spec.DNSServers = []ipamv1.IPAddressStr{"8.8.8.8"}
	existingIPool.SetAnnotations(map[string]string{SearchDomainsKey: "example.com"})
	Expect(tm.GetClient().Update(ctx, existingIPool)).To(Succeed())

	By("VSphereMachine reconcile should configure nameserver and searchDomain values from the IPPool," +
		" in the VSphereMachine's network device, overriding the default values")
	testVSphereMachineReconcileSuccess(getReconcileRequest(vSphereMachineName, tm.VSphereMachine.Namespace))

	updatedCPVSphereMachine = &infrav1.VSphereMachine{}
	Expect(tm.GetClient().Get(ctx, client.ObjectKey{
		Namespace: tm.VSphereMachine.Namespace,
		Name:      vSphereMachineName,
	}, updatedCPVSphereMachine)).To(Succeed())
	Expect(updatedCPVSphereMachine.Spec.Network.Devices[0].Nameservers[0]).To(Equal("8.8.8.8"))
	Expect(updatedCPVSphereMachine.Spec.Network.Devices[0].SearchDomains[0]).To(Equal("example.com"))
}

func verifyVSphereClusterKubeVipAllocation() {
	logInfoLine("verifyVSphereClusterKubeVipAllocation")

	vSphereClusterName := tm.VSphereCluster.Name
	vSphereClusterNamespace := tm.VSphereCluster.Namespace

	By("creation of vsphere cluster with control plane endpoint should succeed")
	Expect(tm.GetClient().Create(ctx, tm.VSphereCluster.DeepCopy())).To(Succeed())

	By("reconcile of vsphere cluster with control plane endpoint should skip static IP Allocation")
	result, err := vSphereClusterReconciler.Reconcile(ctx, getReconcileRequest(vSphereClusterName, vSphereClusterNamespace))
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())

	By("ipam reconcile without an IPClaim should skip creation of IPAddress")
	existingIPClaimList := &ipamv1.IPClaimList{}
	Expect(tm.GetClient().List(ctx, existingIPClaimList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(len(existingIPClaimList.Items)).To(Equal(2))
	result, err = m3ipamReconciler.Reconcile(ctx, ipamctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())
	ipList := &ipamv1.IPAddressList{}
	Expect(tm.GetClient().List(ctx, ipList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(err).To(BeNil())
	Expect(len(ipList.Items)).To(Equal(2))

	By("updating vsphere cluster with ip-pool-name label and empty control plane endpoint should succeed")
	vSphereCluster := &infrav1.VSphereCluster{}
	Expect(tm.GetClient().Get(ctx, client.ObjectKey{
		Namespace: vSphereClusterNamespace,
		Name:      vSphereClusterName,
	}, vSphereCluster)).To(Succeed())

	vSphereCluster.SetLabels(map[string]string{LabelIPPoolName: "ip-pool-pool1"})
	vSphereCluster.Spec.ControlPlaneEndpoint.Host = ""
	Expect(tm.GetClient().Update(ctx, vSphereCluster)).To(Succeed())

	By("first vsphere cluster reconcile should create IPClaim and wait for IPAddress")
	result, err = vSphereClusterReconciler.Reconcile(ctx, getReconcileRequest(vSphereClusterName, vSphereClusterNamespace))
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(Not(BeZero()))
	ipClaimList := &ipamv1.IPClaimList{}
	Expect(tm.GetClient().List(ctx, ipClaimList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(len(ipClaimList.Items)).To(Equal(3))

	By("ipam reconcile should create an IPAddress for the existing IPClaim")
	existingIPAddressList := &ipamv1.IPAddressList{}
	Expect(tm.GetClient().List(ctx, existingIPAddressList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(err).To(BeNil())
	Expect(len(existingIPAddressList.Items)).To(Equal(2))
	result, err = m3ipamReconciler.Reconcile(ctx, ipamctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())
	ipAddressList := &ipamv1.IPAddressList{}
	Expect(tm.GetClient().List(ctx, ipAddressList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(err).To(BeNil())
	Expect(len(ipAddressList.Items)).To(Equal(3))

	By("second vsphere cluster reconcile should allocate the IPAddress to the vsphere cluster's control plane endpoint")
	result, err = vSphereClusterReconciler.Reconcile(ctx, getReconcileRequest(vSphereClusterName, vSphereClusterNamespace))
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())

	updatedVSphereCluster := &infrav1.VSphereCluster{}
	Expect(tm.GetClient().Get(ctx, client.ObjectKey{
		Namespace: vSphereClusterNamespace,
		Name:      vSphereClusterName,
	}, updatedVSphereCluster)).To(Succeed())
	Expect(updatedVSphereCluster.Spec.ControlPlaneEndpoint.Host).To(Equal("10.10.100.22"))
}

func createNewVSphereMachine(name string, isMaster bool, template *infrav1.VSphereMachineTemplate) {
	machine := tm.Machine.DeepCopy()
	machine.Name = name
	machine.SetResourceVersion("")
	Expect(tm.GetClient().Create(ctx, machine)).To(Succeed())

	vSphereMachine := tm.VSphereMachine.DeepCopy()
	vSphereMachine.Name = name
	if isMaster {
		vSphereMachine.SetLabels(map[string]string{LabelControlPlane: ""})
	}

	//set 'clone-from-name' annotation, which is used to select the IPPool for the VM
	vSphereMachine.SetAnnotations(map[string]string{capiv1alpha3.TemplateClonedFromNameAnnotation: template.Name})
	vSphereMachine.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "cluster.x-k8s.io/v1alpha3",
			Kind:       "Machine",
			Name:       machine.Name,
			UID:        machine.UID,
		},
	})

	devices := vSphereMachine.Spec.VirtualMachineCloneSpec.Network.Devices
	for i := range devices {
		devices[i].DHCP4 = false
		devices[i].DHCP6 = false
		devices[i].Nameservers = template.Spec.Template.Spec.Network.Devices[0].Nameservers
		devices[i].SearchDomains = template.Spec.Template.Spec.Network.Devices[0].SearchDomains
	}

	Expect(tm.GetClient().Create(ctx, vSphereMachine)).To(Succeed())
}

func testVSphereMachineReconcileRequeue(req reconcile.Request) {
	result, err := vSphereMachineReconciler.Reconcile(ctx, req)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(Not(BeZero()))
}

func testVSphereMachineReconcileSuccess(req reconcile.Request) {
	result, err := vSphereMachineReconciler.Reconcile(ctx, req)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())
}

func getReconcileRequest(name, namespace string) reconcile.Request {
	return ctrl.Request{
		NamespacedName: client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		},
	}
}
