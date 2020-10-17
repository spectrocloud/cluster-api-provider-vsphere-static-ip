# Cluster API Provider vSphere Static IP

This repository contains controllers to integrate [Kubernetes Cluster API Provider vSphere](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere) with the different IPAM solutions. [Metal3 IP Address Manager](https://github.com/metal3-io/ip-address-manager) is currently the default IPAM solution that is integrated.

## Compatibility with Cluster API

| M3IPAM version    | CAPV version     | Cluster API version |
|-------------------|-------------------|---------------------|
| v1alpha1 (v0.0.4) | v1alpha3 (v0.7.1) | v1alpha3 (v0.3.X)   |
 
## Documentation

See the [Workflow Documentation](docs/workflow.md) for a description of the end-to-end flow.
 
## Deployment

### Deploy cluster API Provider vSphere static IP

```sh
    make deploy
```

### Run locally

```sh
    kubectl scale -n capv-system deployment.v1.apps/capv-static-ip-controller-manager \
      --replicas 0
    make run
```
 