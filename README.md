# cluster-api-provider-vsphere-static-ip
Static IP support for cluster-api-provider-vsphere(CAPV).

CAPV has support to assign a static IP to the VSphereMachine. But CAPI does not support using a MachineDeployment to generate a VSphereMachine with static IP.
If DHCP is disabled, the CAPV controllers will wait for static IPs to be available and then continue to provision VMs.

CAPV-static-ip controller fetches static IPs from the available IP pools and allocates these static IPs to the VSphereMachines.
