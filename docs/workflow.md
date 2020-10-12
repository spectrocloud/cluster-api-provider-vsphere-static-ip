# Workflow

## Requirements

The following controllers need to be deployed:

* CAPI
* CAPV
* CAPM3 IPAM

The cluster-api-provider-static-ip controller has a dependency on:
* Cluster API - *Cluster* and *Machine* objects
* Cluster API vSphere - *HAProxyLoadBalancer*, *VSphereCluster* and *VSphereMachine* objects
* Metal3io IPAM - *IPPool*, *IPClaim* and *IPAddress* objects

At least one valid m3ippool is required.
````
apiVersion: ipam.metal3.io/v1alpha1
kind: IPPool
metadata:
  name: ip-pool-pool1
  namespace: default
  labels:
    cluster.x-k8s.io/ip-pool-group: dev-cluster1-masterpool-vsan-cluster
    cluster.x-k8s.io/network-name: vm-network
spec:
  clusterName: capi-quickstart
  pools:
    - start: 10.10.100.20
      end: 10.10.100.30
      prefix: 18
      gateway: 10.10.100.1
  prefix: 18
  gateway: 10.10.100.1
  namePrefix: capi-quickstart
  dnsServers: [8.8.8.8]
  ````

The CAPV VSphere resources waiting for the static IPs are assigned IPs from a specific IPPool. 
The cluster-api-provider-static-ip can select this IPPool in one of these two ways:
1) The VSphere resource has the label "cluster.x-k8s.io/ip-pool-name", with the value set to the actual IPPool name.
2) The VSphere resource and the IPPool, both have the same match-labels. The current list of supported match-labels are: 
    * "cluster.x-k8s.io/ip-pool-group" - Examples values: dev, prod, dev-cluster1-masterpool, dev-cluster1-masterpool-vsan-cluster  
    * "cluster.x-k8s.io/network-name" - Examples values: vm-network
     
 
## Deployment

Generate manifests 
````
make manifests
````


Deploy manifests 
````
kubectl apply -f _build/manifests/staticip-manifest.yaml
```` 

## Workflow

Example workflow using IPPool name:

Launch a vSphere cluster using [Cluster API](https://cluster-api.sigs.k8s.io/user/quick-start.html) with the required 
changes in the manifest 'capi-quickstart.yaml'.
 * To assign static IPs to the vSphere machines/nodes - set DHCP4 and DHCP6 flags to 'false' in VSphereMachineTemplate
    and set the "cluster.x-k8s.io/ip-pool-name" label to the IPPool that was previously created.
     
 ````
    ---
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
    kind: VSphereMachineTemplate
    metadata:
      labels:
        cluster.x-k8s.io/ip-pool-name: ip-pool-pool1
      name: capi-quickstart
      namespace: default
    spec:
      template:
        spec:
          ...
          network:
            devices:
            - dhcp4: false
              dhcp6: false
              networkName: VM Network
          numCPUs: 2
          ...
          template: ubuntu-1804-kube-v1.17.3
 ````

When the DHCP flags are 'false', the [cluster-api-provider-vsphere](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere) 
VSphereVM controller waits for static IP to be assigned.
Based on the set labels, the  cluster-api-provider-static-ip then selects a matching IPPool and assigns the IP to the
VSphereMachine.

 * To assign a VIP for the cluster with a static IP selected from the IPPool - set an empty value("") in the 
   VSphereCluster's 'controlPlaneEndpoint' and set the "cluster.x-k8s.io/ip-pool-name" label to the IPPool that was 
   previously created.
   
```
    ---
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
    kind: VSphereCluster
    metadata:
      labels:
        cluster.x-k8s.io/ip-pool-name: ip-pool-pool1
      name: capi-quickstart
      namespace: default
    spec:
      cloudProviderConfiguration:
            ...
      controlPlaneEndpoint:
        host: ""
        port: 6443
```

Similarly, match-labels can be used to select the IPPools.