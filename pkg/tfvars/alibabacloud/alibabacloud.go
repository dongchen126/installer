package alibabacloud

import (
	"encoding/json"

	"github.com/openshift/installer/pkg/types"
	"github.com/pkg/errors"
)

// Auth is the collection of credentials that will be used by terrform.
type Auth struct {
	AccessKey string `json:"ali_access_key"`
	SecretKey string `json:"ali_secret_key"`
}

type config struct {
	Auth                  `json:",inline"`
	Region                string         `json:"ali_region_id"`
	ZoneIDs               []string       `json:"ali_zone_ids"`
	ResourceGroupID       string         `json:"ali_resource_group_id"`
	BootstrapInstanceType string         `json:"ali_bootstrap_instance_type"`
	MasterInstanceType    string         `json:"ali_master_instance_type"`
	ImageID               string         `json:"ali_image_id"`
	SystemDiskSize        string         `json:"ali_system_disk_size"`
	SystemDiskCategory    string         `json:"ali_system_disk_category"`
	KeyName               string         `json:"ali_key_name"`
	Tags                  []*InstanceTag `json:"ali_resource_tags"`
	IgnitionBucket        string         `json:"ali_ignition_bucket"`
	BootstrapIgnitionStub string         `json:"ali_bootstrap_stub_ignition"`
}

// TFVarsSources contains the parameters to be converted into Terraform variables
type TFVarsSources struct {
	Auth                  Auth
	ResourceGroupID       string
	BaseDomain            string
	MasterConfigs         []*MachineProviderSpec
	WorkerConfigs         []*MachineProviderSpec
	IgnitionBucket        string
	IgnitionPresignedURL  string
	IgnitionFile          string
	ImageID               string
	SSHKey                string
	Publish               types.PublishingStrategy
	AdditionalTrustBundle string
	Architecture          types.Architecture
}

// TFVars generates AlibabaCloud-specific Terraform variables launching the cluster.
func TFVars(sources TFVarsSources) ([]byte, error) {
	masterConfig := sources.MasterConfigs[0]
	workerConfig := sources.WorkerConfigs[0]

	zoneIDs := make([]string, len(sources.MasterConfigs))
	for i, c := range sources.MasterConfigs {
		zoneIDs[i] = c.ZoneID
	}

	cfg := &config{
		Auth:                  sources.Auth,
		Region:                masterConfig.RegionID,
		ZoneIDs:               zoneIDs,
		ResourceGroupID:       sources.ResourceGroupID,
		BootstrapInstanceType: masterConfig.InstanceType,
		MasterInstanceType:    masterConfig.InstanceType,
		ImageID:               masterConfig.ImageID,
		SystemDiskSize:        masterConfig.SystemDiskSize,
		SystemDiskCategory:    masterConfig.SystemDiskCategory,
		KeyName:               workerConfig.KeyPairName,
		Tags:                  masterConfig.Tag,
		IgnitionBucket:        sources.IgnitionBucket,
	}

	stubIgn, err := generateIgnitionShim(sources.IgnitionPresignedURL, sources.AdditionalTrustBundle)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stub Ignition config for bootstrap")
	}
	cfg.BootstrapIgnitionStub = stubIgn

	return json.MarshalIndent(cfg, "", "  ")
}
