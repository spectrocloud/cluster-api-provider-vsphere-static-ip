package manager

import (
	"path/filepath"

	ipam "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/gomega"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/tests/integration/testenv"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	capivsphere "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	capiv1alpha4 "sigs.k8s.io/cluster-api/api/v1alpha4"
	kubeadmv4 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	CapiCRD   = filepath.Join("..", "integration", "crds", "capi")
	CapvCRD   = filepath.Join("..", "integration", "crds", "capv")
	M3IpamCRD = filepath.Join("..", "integration", "crds", "capm3")
)

type TestManager interface {
	LoadTestEnv()
	InitEnvironment()
}

type Manager struct {
	*testenv.TestData
	env Env
}

type Env struct {
	testEnv *envtest.Environment
	config  *rest.Config
	client  client.Client
}

func (m *Manager) LoadTestEnv() {
	// init test data
	testData, err := testenv.GetTestData()
	Expect(err).NotTo(HaveOccurred())

	m.TestData = testData
}

type InitEnvironmentInput struct {
	Name string
	CRDs []string
}

func (m *Manager) InitEnvironment(input InitEnvironmentInput) {
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: input.CRDs,
	}

	//+kubebuilder:scaffold:scheme
	err := capiv1alpha4.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = capivsphere.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = ipam.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())
	err = kubeadmv4.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())

	cfg, _ := testEnv.Start()
	Expect(cfg).ToNot(BeNil())
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())
	m.env = Env{
		testEnv: testEnv,
		config:  cfg,
		client:  k8sClient,
	}
}

func (m *Manager) GetClient() client.Client {
	return m.env.client
}

func (m *Manager) SaveKubeconfig(path string) {
	managerEnv := m.env
	servers := make(map[string]*clientcmdapi.Cluster)
	localCluster := &clientcmdapi.Cluster{Server: managerEnv.config.Host}
	servers["local"] = localCluster

	contextsC := make(map[string]*clientcmdapi.Context)
	localContextC := &clientcmdapi.Context{Cluster: "local"}
	contextsC["integration"] = localContextC
	configC := clientcmdapi.Config{Kind: "Config", Clusters: servers, Contexts: contextsC, CurrentContext: "integration"}
	err := clientcmd.WriteToFile(configC, path)
	Expect(err).To(Not(HaveOccurred()))
}

func (m *Manager) DestroyEnvironment() {
	Expect(m.env.testEnv.Stop()).ToNot(HaveOccurred())
}

func NewTestManager() *Manager {
	env := Env{}
	return &Manager{
		env: env,
	}
}
