package integration

import (
	"os"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/metal3-io/ip-address-manager/controllers"
	"github.com/metal3-io/ip-address-manager/ipam"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capivsphere "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	kubeadmv3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
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
			Namespace: "default",
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
	Expect(tm.GetClient().Create(ctx, tm.M3IpamIPPool.DeepCopy())).To(Succeed())

	By("creation of capi cluster should succeed")
	Expect(tm.GetClient().Create(ctx, tm.Cluster.DeepCopy())).To(Succeed())
}

func verifyVSphereMachineStaticIPAllocation() {
	logInfoLine("verifyVSphereMachineStaticIPAllocation")

	By("creation of machine should succeed")
	Expect(tm.GetClient().Create(ctx, tm.Machine.DeepCopy())).To(Succeed())

	By("creation of vsphere machine with DHCP set to true should succeed")
	Expect(tm.GetClient().Create(ctx, tm.VSphereMachine.DeepCopy())).To(Succeed())

	By("reconcile of vsphere machine with DHCP should skip static IP Allocation")
	result, err := vSphereMachineReconciler.Reconcile(ctrl.Request{
		NamespacedName: client.ObjectKey{
			Namespace: "default",
			Name:      tm.VSphereMachine.Name,
		},
	})
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())

	By("ipam reconcile without any IPClaim should skip creation of IPAddress")
	result, err = m3ipamReconciler.Reconcile(ipamctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())
	ipList := &ipamv1.IPAddressList{}
	Expect(tm.GetClient().List(ctx, ipList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(err).To(BeNil())
	Expect(len(ipList.Items)).To(Equal(0))

	cpTemplateName := "control-plane-template"

	By("creation of control-plane vsphere machine template with ip-pool-name label should succeed")
	cpVSphereMachineTemp := tm.VSphereMachineTemplate.DeepCopy()
	cpVSphereMachineTemp.Name = cpTemplateName
	cpVSphereMachineTemp.SetLabels(map[string]string{LabelIPPoolName: "ip-pool-pool1"})
	Expect(tm.GetClient().Create(ctx, cpVSphereMachineTemp)).To(Succeed())

	By("creation of kubeadm control plane should succeed")
	kcp := tm.KubeadmControlPlane.DeepCopy()
	kcp.Spec.InfrastructureTemplate.Name = cpTemplateName
	Expect(tm.GetClient().Create(ctx, kcp)).To(Succeed())

	By("creation of control-plane vsphere machine with DHCP set to false should succeed")
	machine := tm.Machine.DeepCopy()
	machine.Name = "cp-machine"
	Expect(tm.GetClient().Create(ctx, machine)).To(Succeed())

	vSphereMachineName := "cp-vsphere-machine"
	vmctrlreq := ctrl.Request{
		NamespacedName: client.ObjectKey{
			Namespace: "default",
			Name:      vSphereMachineName,
		},
	}
	cpVSphereMachine := tm.VSphereMachine.DeepCopy()
	cpVSphereMachine.Name = vSphereMachineName
	cpVSphereMachine.SetLabels(map[string]string{LabelControlPlane: ""})
	cpVSphereMachine.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "cluster.x-k8s.io/v1alpha3",
			Kind:       "Machine",
			Name:       machine.Name,
			UID:        machine.UID,
		},
	})

	devices := cpVSphereMachine.Spec.VirtualMachineCloneSpec.Network.Devices
	updatedDevices := []infrav1.NetworkDeviceSpec{}
	for _, dev := range devices {
		dev.DHCP4 = false
		dev.DHCP6 = false
		updatedDevices = append(updatedDevices, dev)
	}
	cpVSphereMachine.Spec.VirtualMachineCloneSpec.Network.Devices = updatedDevices
	Expect(tm.GetClient().Create(ctx, cpVSphereMachine)).To(Succeed())

	By("first vsphere machine reconcile should create IPClaim and wait for IPAddress")
	result, err = vSphereMachineReconciler.Reconcile(vmctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(Not(BeZero()))
	ipClaimList := &ipamv1.IPClaimList{}
	Expect(tm.GetClient().List(ctx, ipClaimList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(len(ipClaimList.Items)).To(Equal(1))

	By("ipam reconcile should create an IPAddress for the existing IPClaim")
	result, err = m3ipamReconciler.Reconcile(ipamctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())
	ipAddressList := &ipamv1.IPAddressList{}
	Expect(tm.GetClient().List(ctx, ipAddressList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(err).To(BeNil())
	Expect(len(ipAddressList.Items)).To(Equal(1))
	Expect(string(ipAddressList.Items[0].Spec.Address)).To(Equal("10.10.100.20"))

	By("second vsphere machine reconcile should allocated the IPAddress to the waiting vsphere machine")
	result, err = vSphereMachineReconciler.Reconcile(vmctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())

	updatedCPVSphereMachine := &infrav1.VSphereMachine{}
	Expect(tm.GetClient().Get(ctx, client.ObjectKey{
		Namespace: cpVSphereMachine.Namespace,
		Name:      cpVSphereMachine.Name,
	}, updatedCPVSphereMachine)).To(Succeed())
	updatedCPVSphereMachineNw := updatedCPVSphereMachine.Spec.Network
	Expect(updatedCPVSphereMachineNw.Devices[0].IPAddrs[0]).To(Equal("10.10.100.20/18"))
}

func verifyVSphereClusterKubeVipAllocation() {
	logInfoLine("verifyVSphereClusterKubeVipAllocation")

	vSphereClusterName := tm.VSphereCluster.Name
	vSphereClusterNamespace := tm.VSphereCluster.Namespace

	By("creation of vsphere cluster with control plane endpoint should succeed")
	Expect(tm.GetClient().Create(ctx, tm.VSphereCluster.DeepCopy())).To(Succeed())

	By("reconcile of vsphere cluster with control plane endpoint should skip static IP Allocation")
	result, err := vSphereClusterReconciler.Reconcile(ctrl.Request{
		NamespacedName: client.ObjectKey{
			Namespace: "default",
			Name:      vSphereClusterName,
		},
	})
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())

	By("ipam reconcile without an IPClaim should skip creation of IPAddress")
	existingIPClaimList := &ipamv1.IPClaimList{}
	Expect(tm.GetClient().List(ctx, existingIPClaimList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(len(existingIPClaimList.Items)).To(Equal(1))
	result, err = m3ipamReconciler.Reconcile(ipamctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())
	ipList := &ipamv1.IPAddressList{}
	Expect(tm.GetClient().List(ctx, ipList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(err).To(BeNil())
	Expect(len(ipList.Items)).To(Equal(1))

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
	vcctrlreq := ctrl.Request{
		NamespacedName: client.ObjectKey{
			Namespace: "default",
			Name:      vSphereClusterName,
		},
	}

	result, err = vSphereClusterReconciler.Reconcile(vcctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(Not(BeZero()))
	ipClaimList := &ipamv1.IPClaimList{}
	Expect(tm.GetClient().List(ctx, ipClaimList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(len(ipClaimList.Items)).To(Equal(2))

	By("ipam reconcile should create an IPAddress for the existing IPClaim")
	existingIPAddressList := &ipamv1.IPAddressList{}
	Expect(tm.GetClient().List(ctx, existingIPAddressList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(err).To(BeNil())
	Expect(len(existingIPAddressList.Items)).To(Equal(1))
	result, err = m3ipamReconciler.Reconcile(ipamctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())
	ipAddressList := &ipamv1.IPAddressList{}
	Expect(tm.GetClient().List(ctx, ipAddressList, client.InNamespace(tm.Cluster.Namespace))).To(Succeed())
	Expect(err).To(BeNil())
	Expect(len(ipAddressList.Items)).To(Equal(2))

	By("second vsphere cluster reconcile should allocated the IPAddress to the vsphere cluster's control plane endpoint")
	result, err = vSphereClusterReconciler.Reconcile(vcctrlreq)
	Expect(err).To(BeNil())
	Expect(result.RequeueAfter).To(BeZero())

	updatedVSphereCluster := &infrav1.VSphereCluster{}
	Expect(tm.GetClient().Get(ctx, client.ObjectKey{
		Namespace: vSphereClusterNamespace,
		Name:      vSphereClusterName,
	}, updatedVSphereCluster)).To(Succeed())
	Expect(updatedVSphereCluster.Spec.ControlPlaneEndpoint.Host).To(Equal("10.10.100.21"))
}
