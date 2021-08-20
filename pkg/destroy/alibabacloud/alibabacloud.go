package alibabacloud

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/alidns"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/pvtz"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ram"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/slb"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/tag"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	icalibabacloud "github.com/openshift/installer/pkg/asset/installconfig/alibabacloud"
	"github.com/openshift/installer/pkg/destroy/providers"
	"github.com/openshift/installer/pkg/types"
)

// ClusterUninstaller holds the various options for the cluster we want to delete
type ClusterUninstaller struct {
	Logger          logrus.FieldLogger
	Auth            auth.Credential
	Region          string
	InfraID         string
	ClusterID       string
	ClusterDomain   string
	ResourceGroupID string

	ecsClient  *ecs.Client
	dnsClient  *alidns.Client
	pvtzClient *pvtz.Client
	vpcClient  *vpc.Client
	ramClient  *ram.Client
	tagClient  *tag.Client
	slbClient  *slb.Client
}

// ResourceArn holds the information contained in the cloud resource Arn string
type ResourceArn struct {
	Service      string
	Region       string
	Account      string
	ResourceType string
	ResourceID   string
	Arn          string
}

func (o *ClusterUninstaller) configureClients() error {
	var err error
	config := sdk.NewConfig()

	o.ecsClient, err = ecs.NewClientWithOptions(o.Region, config, o.Auth)
	if err != nil {
		return err
	}

	o.dnsClient, err = alidns.NewClientWithOptions(o.Region, config, o.Auth)
	if err != nil {
		return err
	}

	o.pvtzClient, err = pvtz.NewClientWithOptions(o.Region, config, o.Auth)
	if err != nil {
		return err
	}

	o.ramClient, err = ram.NewClientWithOptions(o.Region, config, o.Auth)
	if err != nil {
		return err
	}

	o.vpcClient, err = vpc.NewClientWithOptions(o.Region, config, o.Auth)
	if err != nil {
		return err
	}

	o.tagClient, err = tag.NewClientWithOptions(o.Region, config, o.Auth)
	if err != nil {
		return err
	}

	o.slbClient, err = slb.NewClientWithOptions(o.Region, config, o.Auth)
	if err != nil {
		return err
	}

	return nil
}

// New returns an Alibaba Cloud destroyer from ClusterMetadata.
func New(logger logrus.FieldLogger, metadata *types.ClusterMetadata) (providers.Destroyer, error) {
	region := metadata.ClusterPlatformMetadata.AlibabaCloud.Region
	client, err := icalibabacloud.NewClient(region)
	if err != nil {
		return nil, err
	}

	auth := credentials.NewAccessKeyCredential(client.AccessKeyID, client.AccessKeySecret)

	return &ClusterUninstaller{
		Logger:        logger,
		Auth:          auth,
		Region:        region,
		ClusterID:     metadata.InfraID,
		ClusterDomain: metadata.AlibabaCloud.ClusterDomain,
	}, nil
}

// Run is the entrypoint to start the uninstall process.
func (o *ClusterUninstaller) Run() error {
	var errs []error
	var err error
	err = o.configureClients()
	if err != nil {
		return err
	}

	err = deleteDNSRecords(*o.dnsClient, o.ClusterDomain)
	if err != nil {
		errs = append(errs, err, errors.Wrap(err, "failed to delete DNS records"))
		return utilerrors.NewAggregate(errs)
	}

	err = deletePrivateZones(*o.pvtzClient, o.ClusterDomain)
	if err != nil {
		errs = append(errs, err, errors.Wrap(err, "failed to delete private zones"))
		return utilerrors.NewAggregate(errs)
	}

	err = deleteRAMPolicys(*o.ramClient, o.InfraID)
	if err != nil {
		errs = append(errs, err, errors.Wrap(err, "failed to delete RAM role policys"))
		return utilerrors.NewAggregate(errs)
	}

	err = deleteRAMRoles(*o.ramClient, o.InfraID)
	if err != nil {
		errs = append(errs, err, errors.Wrap(err, "failed to delete RAM role policys"))
		return utilerrors.NewAggregate(errs)
	}

	tagResources, err := findResourcesByTag(*o.tagClient, o.InfraID)
	if err != nil {
		errs = append(errs, err, errors.Wrap(err, "failed to find resource by tag"))
		return utilerrors.NewAggregate(errs)
	}
	if len(tagResources) > 0 {
		var deletedResources []ResourceArn
		for _, resource := range tagResources {
			arn, err := convertResourceArn(resource.ResourceARN)
			if err != nil {
				errs = append(errs, err)
				return utilerrors.NewAggregate(errs)
			}
			deletedResources = append(deletedResources, arn)
		}

		err = deleteResourceByArn(*o.ecsClient, *o.vpcClient, *o.slbClient, *o.tagClient, deletedResources)
		if err != nil {
			errs = append(errs, err)
			return utilerrors.NewAggregate(errs)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func deleteResourceByArn(ecsClient ecs.Client, vpcClient vpc.Client, slbClient slb.Client, tagClient tag.Client, resourceArns []ResourceArn) (err error) {
	var deletedInstances []ResourceArn
	var deletedVpcs []ResourceArn
	var deletedVSwitchs []ResourceArn
	var deletedEips []ResourceArn
	var deletedSecurityGroups []ResourceArn
	var deletedNatGatways []ResourceArn
	var deletedSlbs []ResourceArn
	var others []ResourceArn

	for _, resourceArn := range resourceArns {
		switch resourceArn.Service {
		case "ecs":
			switch resourceArn.ResourceType {
			case "instance":
				deletedInstances = append(deletedInstances, resourceArn)
			case "securitygroup":
				deletedSecurityGroups = append(deletedSecurityGroups, resourceArn)
			default:
				others = append(others, resourceArn)
			}
		case "vpc":
			switch resourceArn.ResourceType {
			case "vpc":
				deletedVpcs = append(deletedVpcs, resourceArn)
			case "vswitch":
				deletedVSwitchs = append(deletedVSwitchs, resourceArn)
			case "eip":
				deletedEips = append(deletedEips, resourceArn)
			case "natgateway":
				deletedNatGatways = append(deletedNatGatways, resourceArn)
			default:
				others = append(others, resourceArn)
			}
		case "slb":
			switch resourceArn.ResourceType {
			case "instance":
				deletedSlbs = append(deletedSlbs, resourceArn)
			default:
				others = append(others, resourceArn)
			}
		default:
			others = append(others, resourceArn)
		}
	}

	err = deleteEcsInstances(ecsClient, deletedInstances)
	if err != nil {
		return err
	}

	err = deleteSecurityGroups(ecsClient, deletedSecurityGroups)
	if err != nil {
		return err
	}

	err = deleteNatGatways(vpcClient, deletedNatGatways)
	if err != nil {
		return err
	}

	err = deleteEips(vpcClient, deletedEips)
	if err != nil {
		return err
	}

	err = deleteEips(vpcClient, deletedEips)
	if err != nil {
		return err
	}

	err = deleteSlbs(slbClient, deletedSlbs)
	if err != nil {
		return err
	}

	err = deleteVSwitchs(vpcClient, deletedVSwitchs)
	if err != nil {
		return err
	}

	err = deleteVpcs(vpcClient, deletedVpcs)
	if err != nil {
		return err
	}

	err = checkOthers(tagClient, others)
	if err != nil {
		return err
	}

	return nil
}

func checkOthers(tagClient tag.Client, resourceArns []ResourceArn) (err error) {
	var arns []string
	for _, Arn := range resourceArns {
		arns = append(arns, Arn.Arn)
	}

	request := tag.CreateListTagResourcesRequest()
	request.PageSize = "1000"
	request.ResourceARN = &arns
	response, err := tagClient.ListTagResources(request)
	if err != nil {
		return err
	}

	if len(response.TagResources) > 0 {
		notDeletedResources := []string{}
		for _, arn := range response.TagResources {
			notDeletedResources = append(notDeletedResources, arn.ResourceARN)
		}
		return errors.New(fmt.Sprintf("There are undeleted cloud resources '%q'", notDeletedResources))
	}
	return
}

func deleteSlbs(slbClient slb.Client, slbArns []ResourceArn) (err error) {
	var slbIDs []string
	for _, slbArn := range slbArns {
		slbIDs = append(slbIDs, slbArn.ResourceID)
	}

	for _, vSwitchID := range slbIDs {
		err = deleteSlb(slbClient, vSwitchID)
		if err != nil {
			return err
		}
	}

	err = wait.Poll(
		1*time.Second,
		1*time.Minute,
		func() (bool, error) {
			response, err := listSlb(slbClient, slbIDs)
			if err != nil {
				return false, err
			}
			if response.TotalCount == 0 {
				return true, nil
			}
			return false, nil
		},
	)
	return
}

func listSlb(slbClient slb.Client, slbIDs []string) (response *slb.DescribeLoadBalancersResponse, err error) {
	request := slb.CreateDescribeLoadBalancersRequest()
	request.LoadBalancerId = strings.Join(slbIDs, ",")
	response, err = slbClient.DescribeLoadBalancers(request)
	return
}

func deleteSlb(slbClient slb.Client, slbID string) (err error) {
	request := slb.CreateDeleteLoadBalancerRequest()
	request.LoadBalancerId = slbID
	_, err = slbClient.DeleteLoadBalancer(request)
	return
}

func deleteVSwitchs(vpcClient vpc.Client, vSwitchArns []ResourceArn) (err error) {
	var vSwitchIDs []string
	for _, vSwitchArn := range vSwitchArns {
		vSwitchIDs = append(vSwitchIDs, vSwitchArn.ResourceID)
	}
	for _, vSwitchID := range vSwitchIDs {
		err = deleteVSwitch(vpcClient, vSwitchID)
		if err != nil {
			return err
		}
	}

	err = wait.Poll(
		1*time.Second,
		1*time.Minute,
		func() (bool, error) {
			response, err := listVSwitch(vpcClient, vSwitchIDs)
			if err != nil {
				return false, err
			}
			if response.TotalCount == 0 {
				return true, nil
			}
			return false, nil
		},
	)
	return
}

func listVSwitch(vpcClient vpc.Client, vSwitchIDs []string) (response *vpc.DescribeVSwitchesResponse, err error) {
	request := vpc.CreateDescribeVSwitchesRequest()
	request.VSwitchId = strings.Join(vSwitchIDs, ",")
	response, err = vpcClient.DescribeVSwitches(request)
	return
}

func deleteVSwitch(vpcClient vpc.Client, vSwitchID string) (err error) {
	request := vpc.CreateDeleteVSwitchRequest()
	request.VSwitchId = vSwitchID
	_, err = vpcClient.DeleteVSwitch(request)
	return
}

func deleteVpcs(vpcClient vpc.Client, vpcArns []ResourceArn) (err error) {
	var vpcIDs []string
	for _, vpcArn := range vpcArns {
		vpcIDs = append(vpcIDs, vpcArn.ResourceID)
	}
	for _, vpcID := range vpcIDs {
		err = deleteVpc(vpcClient, vpcID)
		if err != nil {
			return err
		}
	}

	err = wait.Poll(
		1*time.Second,
		1*time.Minute,
		func() (bool, error) {
			response, err := listVpc(vpcClient, vpcIDs)
			if err != nil {
				return false, err
			}
			if response.TotalCount == 0 {
				return true, nil
			}
			return false, nil
		},
	)

	return
}

func deleteVpc(vpcClient vpc.Client, vpcID string) (err error) {
	request := vpc.CreateDeleteVpcRequest()
	request.VpcId = vpcID
	_, err = vpcClient.DeleteVpc(request)
	return
}

func listVpc(vpcClient vpc.Client, vpcIDs []string) (response *vpc.DescribeVpcsResponse, err error) {
	request := vpc.CreateDescribeVpcsRequest()
	request.VpcId = strings.Join(vpcIDs, ",")
	response, err = vpcClient.DescribeVpcs(request)
	return
}

func deleteEips(vpcClient vpc.Client, eipArns []ResourceArn) (err error) {
	var eipIDs []string
	for _, eipArn := range eipArns {
		eipIDs = append(eipIDs, eipArn.ResourceID)
	}

	for _, eipID := range eipIDs {
		err = deleteEip(vpcClient, eipID)
		if err != nil {
			return err
		}
	}
	err = wait.Poll(
		2*time.Second,
		2*time.Minute,
		func() (bool, error) {
			response, err := listEip(vpcClient, eipIDs)
			if err != nil {
				return false, err
			}
			if response.TotalCount == 0 {
				return true, nil
			}
			return false, nil
		},
	)
	return err
}

func listEip(vpcClient vpc.Client, eipIDs []string) (response *vpc.DescribeEipAddressesResponse, err error) {
	request := vpc.CreateDescribeEipAddressesRequest()
	request.AllocationId = strings.Join(eipIDs, ",")
	response, err = vpcClient.DescribeEipAddresses(request)
	return response, err
}

func deleteEip(vpcClient vpc.Client, eipID string) (err error) {
	request := vpc.CreateReleaseEipAddressRequest()
	request.AllocationId = eipID
	_, err = vpcClient.ReleaseEipAddress(request)
	return
}

func deleteNatGatways(vpcClient vpc.Client, natGatwayArns []ResourceArn) (err error) {
	var natGatwayIDs []string
	for _, natGatwayArn := range natGatwayArns {
		natGatwayIDs = append(natGatwayIDs, natGatwayArn.ResourceID)
	}
	// TODO: more appropriate to use asynchronous. It is advisable to optimise in the future
	for _, natGatwayID := range natGatwayIDs {
		err = deleteNatGatway(vpcClient, natGatwayID)
		if err != nil {
			return err
		}
		err = wait.Poll(
			3*time.Second,
			3*time.Minute,
			func() (bool, error) {
				response, err := listNatGatways(vpcClient, natGatwayID)
				if err != nil {
					return false, err
				}
				if response.TotalCount == 0 {
					return true, nil
				}
				return false, nil
			},
		)
		if err != nil {
			return err
		}
	}
	return
}

func listNatGatways(vpcClient vpc.Client, natGatwayID string) (response *vpc.DescribeNatGatewaysResponse, err error) {
	request := vpc.CreateDescribeNatGatewaysRequest()
	request.NatGatewayId = natGatwayID
	response, err = vpcClient.DescribeNatGateways(request)
	return
}

func deleteNatGatway(vpcClient vpc.Client, natGatwayID string) (err error) {
	request := vpc.CreateDeleteNatGatewayRequest()
	request.NatGatewayId = natGatwayID
	request.Force = "true"
	_, err = vpcClient.DeleteNatGateway(request)
	return
}

func deleteSecurityGroups(ecsClient ecs.Client, securityGroupArns []ResourceArn) (err error) {
	var securityGroupIDs []string

	for _, securityGroupArn := range securityGroupArns {
		securityGroupIDs = append(securityGroupIDs, securityGroupArn.ResourceID)
	}
	for _, securityGroupID := range securityGroupIDs {
		err = deleteSecurityGroupRules(ecsClient, securityGroupID)
		if err != nil {
			return err
		}
	}

	err = wait.Poll(
		1*time.Second,
		1*time.Minute,
		func() (bool, error) {
			response, err := listSecurityGroupReferences(ecsClient, securityGroupIDs)
			if err != nil {
				return false, err
			}
			if len(response.SecurityGroupReferences.SecurityGroupReference) == 0 {
				return true, nil
			}
			return false, nil
		},
	)
	if err != nil {
		return err
	}

	for _, securityGroupID := range securityGroupIDs {
		err = deleteSecurityGroup(ecsClient, securityGroupID)
		if err != nil {
			return err
		}
	}

	err = wait.Poll(
		1*time.Second,
		1*time.Minute,
		func() (bool, error) {
			response, err := listSecurityGroup(ecsClient, securityGroupIDs)
			if err != nil {
				return false, err
			}
			if response.TotalCount == 0 {
				return true, nil
			}
			return false, nil
		},
	)

	return
}

func deleteSecurityGroup(ecsClient ecs.Client, securityGroupID string) (err error) {
	request := ecs.CreateDeleteSecurityGroupRequest()
	request.SecurityGroupId = securityGroupID
	_, err = ecsClient.DeleteSecurityGroup(request)
	return
}

func deleteSecurityGroupRules(ecsClient ecs.Client, securityGroupID string) (err error) {
	response, err := getSecurityGroup(ecsClient, securityGroupID)
	if err != nil {
		return err
	}
	for _, permission := range response.Permissions.Permission {
		if permission.SourceGroupId != "" {
			err = revokeSecurityGroup(ecsClient, securityGroupID, permission.SourceGroupId, permission.IpProtocol, permission.PortRange, permission.NicType)
		}
	}
	return
}

func revokeSecurityGroup(ecsClient ecs.Client, securityGroupID string, sourceGroupID string, ipProtocol string, portRange string, nicType string) (err error) {
	request := ecs.CreateRevokeSecurityGroupRequest()
	request.SecurityGroupId = securityGroupID
	request.SourceGroupId = sourceGroupID
	request.IpProtocol = ipProtocol
	request.PortRange = portRange
	request.NicType = nicType

	_, err = ecsClient.RevokeSecurityGroup(request)
	return
}

func getSecurityGroup(ecsClient ecs.Client, securityGroupID string) (response *ecs.DescribeSecurityGroupAttributeResponse, err error) {
	request := ecs.CreateDescribeSecurityGroupAttributeRequest()
	request.SecurityGroupId = securityGroupID
	response, err = ecsClient.DescribeSecurityGroupAttribute(request)
	return
}

func listSecurityGroupReferences(ecsClient ecs.Client, securityGroupIDs []string) (response *ecs.DescribeSecurityGroupReferencesResponse, err error) {
	request := ecs.CreateDescribeSecurityGroupReferencesRequest()
	request.SecurityGroupId = &securityGroupIDs
	response, err = ecsClient.DescribeSecurityGroupReferences(request)
	return
}

func listSecurityGroup(ecsClient ecs.Client, securityGroupIDs []string) (response *ecs.DescribeSecurityGroupsResponse, err error) {
	request := ecs.CreateDescribeSecurityGroupsRequest()
	securityGroupIDsString, err := json.Marshal(securityGroupIDs)
	if err != nil {
		return nil, err
	}
	request.SecurityGroupIds = string(securityGroupIDsString)
	response, err = ecsClient.DescribeSecurityGroups(request)
	return
}

func listEcsInstance(ecsClient ecs.Client, instanceIDs []string) (response *ecs.DescribeInstancesResponse, err error) {
	request := ecs.CreateDescribeInstancesRequest()
	instanceIDsString, err := json.Marshal(instanceIDs)
	if err != nil {
		return nil, err
	}
	request.InstanceIds = string(instanceIDsString)
	response, err = ecsClient.DescribeInstances(request)
	return
}

func deleteEcsInstances(ecsClient ecs.Client, instanceArns []ResourceArn) (err error) {
	var instanceIDs []string
	for _, instanceArn := range instanceArns {
		instanceIDs = append(instanceIDs, instanceArn.ResourceID)
	}
	request := ecs.CreateDeleteInstancesRequest()
	request.InstanceId = &instanceIDs
	request.Force = "true"
	_, err = ecsClient.DeleteInstances(request)
	if err != nil {
		return err
	}

	err = wait.Poll(
		5*time.Second,
		5*time.Minute,
		func() (bool, error) {
			response, err := listEcsInstance(ecsClient, instanceIDs)
			if err != nil {
				return false, err
			}
			if response.TotalCount == 0 {
				return true, nil
			}
			return false, nil
		},
	)
	return
}

func findResourcesByTag(tagClient tag.Client, infraID string) (tagResources []tag.TagResource, err error) {
	tags := map[string]string{fmt.Sprintf("kubernetes.io/cluster/%q", infraID): "owned"}
	tagsString, err := json.Marshal(tags)
	if err != nil {
		return nil, err
	}

	request := tag.CreateListTagResourcesRequest()
	request.PageSize = "1000"
	request.Tags = string(tagsString)
	request.Category = "Custom"
	response, err := tagClient.ListTagResources(request)
	if err != nil {
		return nil, err
	}
	return response.TagResources, nil
}

func deleteRAMRoles(ramClient ram.Client, infraID string) (err error) {
	masterRoleName := fmt.Sprintf("%q-role-master", infraID)
	workerRoleName := fmt.Sprintf("%q-role-worker", infraID)

	err = deleteRAMRole(ramClient, masterRoleName)
	if err != nil && !strings.Contains(err.Error(), "EntityNotExist.Role") {
		return err
	}

	err = deleteRAMRole(ramClient, workerRoleName)
	if err != nil && !strings.Contains(err.Error(), "EntityNotExist.Role") {
		return err
	}
	return nil
}

func deleteRAMRole(ramClient ram.Client, roleName string) (err error) {
	request := ram.CreateDeleteRoleRequest()
	request.RoleName = roleName
	_, err = ramClient.DeleteRole(request)
	return
}

func deleteRAMPolicys(ramClient ram.Client, infraID string) (err error) {
	masterPolicyName := fmt.Sprintf("%q-policy-master", infraID)
	err = deletePolicy(ramClient, masterPolicyName)
	if err != nil {
		return err
	}

	workerPolicyName := fmt.Sprintf("%q-policy-worker", infraID)
	err = deletePolicy(ramClient, workerPolicyName)
	if err != nil {
		return err
	}

	err = wait.Poll(
		1*time.Second,
		1*time.Minute,
		func() (bool, error) {
			_, err := getPolicy(ramClient, masterPolicyName)
			if err != nil {
				if strings.Contains(err.Error(), "EntityNotExist.Policy") {
					return true, nil
				}
				return false, err
			}
			return false, nil
		},
	)
	if err != nil {
		return
	}

	err = wait.Poll(
		1*time.Second,
		1*time.Minute,
		func() (bool, error) {
			_, err := getPolicy(ramClient, workerPolicyName)
			if err != nil {
				if strings.Contains(err.Error(), "EntityNotExist.Policy") {
					return true, nil
				}
				return false, err
			}
			return false, nil
		},
	)
	return
}

func getPolicy(ramClient ram.Client, policyName string) (response *ram.GetPolicyResponse, err error) {
	request := ram.CreateGetPolicyRequest()
	request.PolicyName = policyName
	request.PolicyType = "Custom"
	response, err = ramClient.GetPolicy(request)
	return
}

func deletePolicy(ramClient ram.Client, policyName string) (err error) {
	request := ram.CreateDeletePolicyRequest()
	request.PolicyName = policyName
	_, err = ramClient.DeletePolicy(request)
	return
}

func deletePrivateZones(pvtzClient pvtz.Client, clusterDomain string) (err error) {
	zones, err := listPrivateZone(pvtzClient, clusterDomain)
	if err != nil {
		return err
	}
	if len(zones) == 0 {
		return nil
	}
	if len(zones) > 1 {
		return errors.Wrap(err, fmt.Sprintf("matched to multiple private zones by clustedomain '%q'", clusterDomain))
	}

	zoneID := zones[0].ZoneId
	err = bindZoneVpc(pvtzClient, zoneID)
	if err != nil {
		return err
	}

	// Wait for unbind vpc to complete
	err = wait.Poll(
		1*time.Second,
		1*time.Minute,
		func() (bool, error) {
			zones, err := listPrivateZone(pvtzClient, clusterDomain)
			if err != nil {
				return false, err
			}

			if len(zones[0].Vpcs.Vpc) == 0 {
				return true, nil
			}
			return false, nil
		},
	)
	if err != nil {
		return
	}

	// Delete a private zone does not require delete the record in advance
	err = deletePrivateZone(pvtzClient, zoneID)
	if err != nil {
		return err
	}

	// Wait for deletion private zone to complete
	err = wait.Poll(
		1*time.Second,
		1*time.Minute,
		func() (bool, error) {
			zones, err := listPrivateZone(pvtzClient, clusterDomain)
			if err != nil {
				return false, err
			}

			if len(zones) == 0 {
				return true, nil
			}
			return false, nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func deletePrivateZone(pvtzClient pvtz.Client, zoneID string) (err error) {
	request := pvtz.CreateDeleteZoneRequest()
	request.ZoneId = zoneID
	_, err = pvtzClient.DeleteZone(request)
	return
}

func bindZoneVpc(pvtzClient pvtz.Client, zoneID string) (err error) {
	request := pvtz.CreateBindZoneVpcRequest()
	request.ZoneId = zoneID
	_, err = pvtzClient.BindZoneVpc(request)
	return
}

func listPrivateZone(pvtzClient pvtz.Client, clusterDomain string) ([]pvtz.Zone, error) {
	request := pvtz.CreateDescribeZonesRequest()
	request.Lang = "en"
	request.Keyword = clusterDomain

	response, err := pvtzClient.DescribeZones(request)
	if err != nil {
		return nil, err
	}
	return response.Zones.Zone, nil
}

func deleteDNSRecords(dnsClient alidns.Client, clusterDomain string) (err error) {
	baseDomain := strings.Join(strings.Split(clusterDomain, ".")[1:], ".")
	domains, err := listDomain(dnsClient, baseDomain)
	if err != nil {
		return
	}
	if len(domains) == 0 {
		return
	}

	records, err := listRecord(dnsClient, baseDomain)
	if err != nil {
		return
	}
	if len(records) == 0 {
		return
	}

	for _, record := range records {
		err = deleteRecord(dnsClient, record.RecordId)
		if err != nil {
			return
		}
	}

	// Wait for deletion to complete
	err = wait.Poll(
		1*time.Second,
		1*time.Minute,
		func() (bool, error) {
			records, err := listRecord(dnsClient, baseDomain)
			if err != nil {
				return false, err
			}

			if len(records) == 0 {
				return true, nil
			}
			return false, nil
		},
	)
	if err != nil {
		return
	}

	return nil
}

func deleteRecord(dnsclient alidns.Client, recordID string) error {
	request := alidns.CreateDeleteDomainRecordRequest()
	request.Scheme = "https"
	request.RecordId = recordID
	_, err := dnsclient.DeleteDomainRecord(request)
	if err != nil {
		return err
	}
	return nil
}

func listDomain(dnsclient alidns.Client, baseDomain string) ([]alidns.DomainInDescribeDomains, error) {
	request := alidns.CreateDescribeDomainsRequest()
	request.Scheme = "https"
	request.KeyWord = baseDomain
	response, err := dnsclient.DescribeDomains(request)
	if err != nil {
		return nil, err
	}
	return response.Domains.Domain, nil
}

func listRecord(dnsclient alidns.Client, baseDomain string) ([]alidns.Record, error) {
	request := alidns.CreateDescribeDomainRecordsRequest()
	request.Scheme = "https"
	request.DomainName = baseDomain
	response, err := dnsclient.DescribeDomainRecords(request)
	if err != nil {
		return nil, err
	}
	return response.DomainRecords.Record, nil
}

func convertResourceArn(arn string) (resourceArn ResourceArn, err error) {
	_arn := strings.Split(arn, "/")
	serviceInfos := strings.Split(_arn[0], ":")

	resourceArn.Service = serviceInfos[2]
	resourceArn.Region = serviceInfos[3]
	resourceArn.Account = serviceInfos[4]
	resourceArn.ResourceType = serviceInfos[5]
	resourceArn.ResourceID = _arn[1]
	resourceArn.Arn = arn
	return resourceArn, nil
}
