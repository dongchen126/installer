package alibabacloud

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/installer/pkg/types"
	alibabacloudtypes "github.com/openshift/installer/pkg/types/alibabacloud"
)

// Validate executes platform-specific validation.
func Validate(client *Client, ic *types.InstallConfig) error {
	allErrs := field.ErrorList{}
	platformPath := field.NewPath("platform").Child("alibabacloud")
	allErrs = append(allErrs, validatePlatform(client, ic, platformPath)...)

	allErrs = append(allErrs, validateControlPlaneMachinePool(client, ic)...)
	allErrs = append(allErrs, validateComputeMachinePool(client, ic)...)

	return allErrs.ToAggregate()
}

func validateControlPlaneMachinePool(client *Client, ic *types.InstallConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	mpool := mergedMachinePool{}
	defaultPool := alibabacloudtypes.DefaultMasterMachinePoolPlatform()
	mpool.setWithFieldPath(&defaultPool, field.NewPath("controlPlane", "platform", "alibabacloud"))
	if ic.Platform.AlibabaCloud != nil {
		mpool.setWithFieldPath(ic.Platform.AlibabaCloud.DefaultMachinePlatform, field.NewPath("platform", "alibabacloud", "defaultMachinePlatform"))
	}
	mpool.setWithFieldPath(ic.ControlPlane.Platform.AlibabaCloud, field.NewPath("controlPlane", "platform", "alibabacloud"))

	allErrs = append(allErrs, validateMachinePool(client, ic, &mpool)...)
	return allErrs
}

func validateComputeMachinePool(client *Client, ic *types.InstallConfig) field.ErrorList {
	allErrs := field.ErrorList{}

	for idx, compute := range ic.Compute {
		mpool := mergedMachinePool{}
		computePoolFieldPath := field.NewPath("compute").Index(idx).Child("platform", "alibabacloud")
		defaultPool := alibabacloudtypes.DefaultWorkerMachinePoolPlatform()
		mpool.setWithFieldPath(&defaultPool, computePoolFieldPath)
		if ic.Platform.AlibabaCloud != nil {
			mpool.setWithFieldPath(ic.Platform.AlibabaCloud.DefaultMachinePlatform, field.NewPath("platform", "alibabacloud", "defaultMachinePlatform"))
		}

		if compute.Platform.AlibabaCloud != nil {
			mpool.setWithFieldPath(compute.Platform.AlibabaCloud, computePoolFieldPath)
		}
		allErrs = append(allErrs, validateMachinePool(client, ic, &mpool)...)
	}
	return allErrs
}

func validateMachinePool(client *Client, ic *types.InstallConfig, pool *mergedMachinePool) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(pool.Zones) > 0 {
		availableZones := map[string]bool{}

		response, err := client.DescribeAvailableResource("Zone")
		if err != nil {
			return append(allErrs, field.InternalError(pool.zonesFieldPath, err))
		}
		for _, availableZone := range response.AvailableZones.AvailableZone {
			if availableZone.Status == "Available" {
				availableZones[availableZone.ZoneId] = true
			}
		}

		for idx, zone := range pool.Zones {
			if !availableZones[zone] {
				allErrs = append(allErrs, field.Invalid(pool.zonesFieldPath.Index(idx), zone, fmt.Sprintf("zone ID is unavailable in region %q", ic.Platform.AlibabaCloud.Region)))
			}
		}
	}
	// InstanceType and zones are related.
	// If the availability zone is not available, the instanceType will not be validated.
	if len(allErrs) == 0 {
		allErrs = append(allErrs, validateInstanceType(client, pool.Zones, pool.InstanceType, pool.instanceTypeFieldPath)...)
	}

	return allErrs
}

func validateInstanceType(client *Client, zones []string, instanceType string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	availableZones, err := client.GetAvailableZonesByInstanceType(instanceType)

	if err != nil {
		return append(allErrs, field.InternalError(fldPath, err))
	}
	if len(availableZones) == 0 {
		return append(allErrs, field.Invalid(fldPath, instanceType, "no available availability zones found"))
	}

	zonesWithStock := sets.NewString(availableZones...)
	for _, zoneID := range zones {
		if zonesWithStock.Has(zoneID) {
			allErrs = append(allErrs, field.Invalid(fldPath, instanceType, fmt.Sprintf("instance type is out of stock or unavailable in zone %q", zoneID)))
		}
	}
	return allErrs
}

func validatePlatform(client *Client, ic *types.InstallConfig, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateResourceGroup(client, ic, path)...)
	return allErrs
}

func validateResourceGroup(client *Client, ic *types.InstallConfig, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	resourceGroups, err := client.ListResourceGroups()
	if err != nil {
		return append(allErrs, field.InternalError(path.Child("resourceGroupID"), err))
	}
	for _, rg := range resourceGroups.ResourceGroups.ResourceGroup {
		if rg.Id == ic.AlibabaCloud.ResourceGroupID {
			return allErrs
		}
	}
	return append(allErrs, field.NotFound(path.Child("resourceGroupID"), ic.AlibabaCloud.ResourceGroupID))
}

// ValidateForProvisioning validates if the install config is valid for provisioning the cluster.
func ValidateForProvisioning(client *Client, ic *types.InstallConfig, metadata *Metadata) error {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateClusterName(client, ic)...)
	return allErrs.ToAggregate()
}

func validateClusterName(client *Client, ic *types.InstallConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	namePath := field.NewPath("metadata").Child("name")

	zoneName := ic.ClusterDomain()
	response, err := client.ListPrivateZones(zoneName)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(namePath, err))
	}
	if response.TotalItems > 0 {
		allErrs = append(allErrs, field.Invalid(namePath, ic.ObjectMeta.Name, fmt.Sprintf("cluster name is unavailable, private zone name %s already exists", zoneName)))
	}
	return allErrs
}

type mergedMachinePool struct {
	alibabacloudtypes.MachinePool
	zonesFieldPath        *field.Path
	instanceTypeFieldPath *field.Path
}

func (a *mergedMachinePool) setWithFieldPath(required *alibabacloudtypes.MachinePool, fldPath *field.Path) {
	if required == nil || a == nil {
		return
	}

	if len(required.Zones) > 0 {
		a.Zones = required.Zones
		a.zonesFieldPath = fldPath.Child("zones")
	}
	if required.InstanceType != "" {
		a.InstanceType = required.InstanceType
		a.instanceTypeFieldPath = fldPath.Child("instanceType")
	}
}
