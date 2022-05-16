module github.com/spectrocloud/cluster-api-provider-vsphere-static-ip

go 1.16

require (
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/metal3-io/ip-address-manager v0.1.1
	github.com/metal3-io/ip-address-manager/api v0.0.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.7.0
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.9.0
	sigs.k8s.io/cluster-api v0.4.3
	sigs.k8s.io/cluster-api-provider-vsphere v0.8.1
	sigs.k8s.io/controller-runtime v0.10.1
)

replace (
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	github.com/metal3-io/ip-address-manager/api => github.com/metal3-io/ip-address-manager/api v0.0.0-20210929111944-d66dc8cb0347
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.3
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.9.7
)
