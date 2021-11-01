package alibabacloud

// Metadata contains Alibaba Cloud metadata (e.g. for uninstalling the cluster).
type Metadata struct {
	Region          string   `json:"region"`
	ResourceGroupID string   `json:"resourceGroupID"`
	VpcID           string   `json:"vpcID"`
	VSwitchIDs      []string `json:"vswitchIDs"`
	PrivateZoneID   string   `json:"privateZoneID"`
	ClusterDomain   string   `json:"clusterDomain"`
}
