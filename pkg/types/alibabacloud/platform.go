package alibabacloud

// Platform stores all the global configuration that all machinesets use.
type Platform struct {
	// Region specifies the Alibaba Cloud region where the cluster will be created.
	Region string `json:"region"`

	// ResourceGroupID is the ID of an already existing resource group where the cluster should be installed.
	// This resource group must be empty with no other resources when trying to use it for creating a cluster.
	// If empty, a new resource group will created for the cluster.
	// Destroying the cluster using installer will delete this resource group.
	// +optional
	ResourceGroupID string `json:"resourceGroupID"`

	// VpcID is the ID of an already existing VPC where the cluster should be installed.
	// If empty, a new VPC will created for the cluster.
	// Destroying the cluster using installer will delete this VPC.
	// +optional
	VpcID string `json:"vpcID,omitempty"`

	// VSwitchIDs is the ID list of already existing VSwitchs where the master should be created.
	// If empty, the new VSwitchs will created for the cluster.
	// Destroying the cluster using installer will delete these VSwitchs.
	// +optional
	VSwitchIDs []string `json:"vswitchIDs,omitempty"`

	// PrivateZoneID is the ID of an existing private zone into which to add DNS
	// records for the cluster's internal API. An existing private zone can
	// only be used when also using existing vpc. The private zone must be
	// associated with the VPC containing the subnets.
	// Leave the private zone unset to have the installer create the private zone
	// on your behalf.
	// +optional
	PrivateZoneID string `json:"privateZoneID,omitempty"`

	// Tags additional keys and values that the installer will add
	// as tags to all resources that it creates. Resources created by the
	// cluster itself may not include these tags.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`

	// DefaultMachinePlatform is the default configuration used when installing
	// on Alibaba Cloud for machine pools which do not define their own platform
	// configuration.
	// +optional
	DefaultMachinePlatform *MachinePool `json:"defaultMachinePlatform,omitempty"`
}
