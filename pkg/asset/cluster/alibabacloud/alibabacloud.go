package alibabacloud

import (
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/alibabacloud"
)

// Metadata converts an install configuration to Alibaba Cloud metadata.
func Metadata(config *types.InstallConfig) *alibabacloud.Metadata {
	return &alibabacloud.Metadata{
		Region:          config.Platform.AlibabaCloud.Region,
		ResourceGroupID: config.Platform.AlibabaCloud.ResourceGroupID,
		VpcID:           config.Platform.AlibabaCloud.VpcID,
		VSwitchIDs:      config.Platform.AlibabaCloud.VSwitchIDs,
		PrivateZoneID:   config.Platform.AlibabaCloud.PrivateZoneID,
		ClusterDomain:   config.ClusterDomain(),
	}
}
