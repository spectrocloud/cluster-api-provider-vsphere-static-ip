# cluster-api-provider-vsphere-static-ip
Static IP support for cluster-api-provider-vsphere(CAPV).

1) CAPV has support to assign a static IP to the VSphereMachine. But CAPI does not support using a MachineDeployment to generate a VSphereMachine with static IP.
If DHCP is disabled, the CAPV controllers will wait for static IPs to be available and then continue to provision VMs.

2) CAPV uses VIP as the default control plane endpoint. If control plane endpoint is not set in the VSphereCluster, CAPV controllers will wait for the control plane endpoint host to be available. 

The cluster-api-provider-static-ip service provides controllers to fetch static IPs from the available IP pools and to allocate these static IPs to the VSphereCluster and VSphereMachine objects.
