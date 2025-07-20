/*
Copyright 2018 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package awsclient

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/account"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/support"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclientpkg "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	awsCredsSecretIDKeyV2     = "aws_access_key_id"     // #nosec G101 -- This is a false positive
	awsCredsSecretAccessKeyV2 = "aws_secret_access_key" // #nosec G101 -- This is a false positive
)

//go:generate mockgen -source=./client_v2.go -destination=./mock/zz_generated.mock_client_v2.go -package=mock

// ClientV2 is a wrapper object for actual AWS SDK v2 clients to allow for easier testing.
type ClientV2 interface {
	//Account
	EnableRegion(context.Context, *account.EnableRegionInput, ...func(*account.Options)) (*account.EnableRegionOutput, error)
	GetRegionOptStatus(context.Context, *account.GetRegionOptStatusInput, ...func(*account.Options)) (*account.GetRegionOptStatusOutput, error)

	//EC2
	RunInstances(context.Context, *ec2.RunInstancesInput, ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	DescribeInstanceStatus(context.Context, *ec2.DescribeInstanceStatusInput, ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error)
	TerminateInstances(context.Context, *ec2.TerminateInstancesInput, ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	DescribeVolumes(context.Context, *ec2.DescribeVolumesInput, ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	DeleteVolume(context.Context, *ec2.DeleteVolumeInput, ...func(*ec2.Options)) (*ec2.DeleteVolumeOutput, error)
	DescribeSnapshots(context.Context, *ec2.DescribeSnapshotsInput, ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
	DeleteSnapshot(context.Context, *ec2.DeleteSnapshotInput, ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error)
	DescribeImages(context.Context, *ec2.DescribeImagesInput, ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeInstanceTypes(context.Context, *ec2.DescribeInstanceTypesInput, ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error)
	DescribeRegions(context.Context, *ec2.DescribeRegionsInput, ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
	DescribeVpcEndpointServiceConfigurations(context.Context, *ec2.DescribeVpcEndpointServiceConfigurationsInput, ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointServiceConfigurationsOutput, error)
	DeleteVpcEndpointServiceConfigurations(context.Context, *ec2.DeleteVpcEndpointServiceConfigurationsInput, ...func(*ec2.Options)) (*ec2.DeleteVpcEndpointServiceConfigurationsOutput, error)
	DescribeVpcs(context.Context, *ec2.DescribeVpcsInput, ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	CreateVpc(context.Context, *ec2.CreateVpcInput, ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error)
	DeleteVpc(context.Context, *ec2.DeleteVpcInput, ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error)
	DescribeSubnets(context.Context, *ec2.DescribeSubnetsInput, ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	CreateSubnet(context.Context, *ec2.CreateSubnetInput, ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error)
	DeleteSubnet(context.Context, *ec2.DeleteSubnetInput, ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error)

	//IAM
	CreateAccessKey(context.Context, *iam.CreateAccessKeyInput, ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error)
	CreateUser(context.Context, *iam.CreateUserInput, ...func(*iam.Options)) (*iam.CreateUserOutput, error)
	DeleteAccessKey(context.Context, *iam.DeleteAccessKeyInput, ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error)
	DeleteUser(context.Context, *iam.DeleteUserInput, ...func(*iam.Options)) (*iam.DeleteUserOutput, error)
	DeleteUserPolicy(context.Context, *iam.DeleteUserPolicyInput, ...func(*iam.Options)) (*iam.DeleteUserPolicyOutput, error)
	GetUser(context.Context, *iam.GetUserInput, ...func(*iam.Options)) (*iam.GetUserOutput, error)
	ListUsers(context.Context, *iam.ListUsersInput, ...func(*iam.Options)) (*iam.ListUsersOutput, error)
	ListUserTags(context.Context, *iam.ListUserTagsInput, ...func(*iam.Options)) (*iam.ListUserTagsOutput, error)
	ListAccessKeys(context.Context, *iam.ListAccessKeysInput, ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
	ListUserPolicies(context.Context, *iam.ListUserPoliciesInput, ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error)
	PutUserPolicy(context.Context, *iam.PutUserPolicyInput, ...func(*iam.Options)) (*iam.PutUserPolicyOutput, error)
	AttachUserPolicy(context.Context, *iam.AttachUserPolicyInput, ...func(*iam.Options)) (*iam.AttachUserPolicyOutput, error)
	DetachUserPolicy(context.Context, *iam.DetachUserPolicyInput, ...func(*iam.Options)) (*iam.DetachUserPolicyOutput, error)
	ListPolicies(context.Context, *iam.ListPoliciesInput, ...func(*iam.Options)) (*iam.ListPoliciesOutput, error)
	ListAttachedUserPolicies(context.Context, *iam.ListAttachedUserPoliciesInput, ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error)
	CreatePolicy(context.Context, *iam.CreatePolicyInput, ...func(*iam.Options)) (*iam.CreatePolicyOutput, error)
	DeletePolicy(context.Context, *iam.DeletePolicyInput, ...func(*iam.Options)) (*iam.DeletePolicyOutput, error)
	DeletePolicyVersion(context.Context, *iam.DeletePolicyVersionInput, ...func(*iam.Options)) (*iam.DeletePolicyVersionOutput, error)
	GetPolicy(context.Context, *iam.GetPolicyInput, ...func(*iam.Options)) (*iam.GetPolicyOutput, error)
	GetPolicyVersion(context.Context, *iam.GetPolicyVersionInput, ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error)
	ListPolicyVersions(context.Context, *iam.ListPolicyVersionsInput, ...func(*iam.Options)) (*iam.ListPolicyVersionsOutput, error)
	AttachRolePolicy(context.Context, *iam.AttachRolePolicyInput, ...func(*iam.Options)) (*iam.AttachRolePolicyOutput, error)
	DetachRolePolicy(context.Context, *iam.DetachRolePolicyInput, ...func(*iam.Options)) (*iam.DetachRolePolicyOutput, error)
	ListAttachedRolePolicies(context.Context, *iam.ListAttachedRolePoliciesInput, ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error)
	ListRolePolicies(context.Context, *iam.ListRolePoliciesInput, ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error)
	DeleteRolePolicy(context.Context, *iam.DeleteRolePolicyInput, ...func(*iam.Options)) (*iam.DeleteRolePolicyOutput, error)
	CreateRole(context.Context, *iam.CreateRoleInput, ...func(*iam.Options)) (*iam.CreateRoleOutput, error)
	GetRole(context.Context, *iam.GetRoleInput, ...func(*iam.Options)) (*iam.GetRoleOutput, error)
	DeleteRole(context.Context, *iam.DeleteRoleInput, ...func(*iam.Options)) (*iam.DeleteRoleOutput, error)
	ListRoles(context.Context, *iam.ListRolesInput, ...func(*iam.Options)) (*iam.ListRolesOutput, error)
	PutRolePolicy(context.Context, *iam.PutRolePolicyInput, ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)

	//Organizations
	ListAccounts(context.Context, *organizations.ListAccountsInput, ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error)
	CreateAccount(context.Context, *organizations.CreateAccountInput, ...func(*organizations.Options)) (*organizations.CreateAccountOutput, error)
	DescribeCreateAccountStatus(context.Context, *organizations.DescribeCreateAccountStatusInput, ...func(*organizations.Options)) (*organizations.DescribeCreateAccountStatusOutput, error)
	ListCreateAccountStatus(context.Context, *organizations.ListCreateAccountStatusInput, ...func(*organizations.Options)) (*organizations.ListCreateAccountStatusOutput, error)
	MoveAccount(context.Context, *organizations.MoveAccountInput, ...func(*organizations.Options)) (*organizations.MoveAccountOutput, error)
	CreateOrganizationalUnit(context.Context, *organizations.CreateOrganizationalUnitInput, ...func(*organizations.Options)) (*organizations.CreateOrganizationalUnitOutput, error)
	ListOrganizationalUnitsForParent(context.Context, *organizations.ListOrganizationalUnitsForParentInput, ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error)
	ListChildren(context.Context, *organizations.ListChildrenInput, ...func(*organizations.Options)) (*organizations.ListChildrenOutput, error)
	TagResource(context.Context, *organizations.TagResourceInput, ...func(*organizations.Options)) (*organizations.TagResourceOutput, error)
	UntagResource(context.Context, *organizations.UntagResourceInput, ...func(*organizations.Options)) (*organizations.UntagResourceOutput, error)
	ListParents(context.Context, *organizations.ListParentsInput, ...func(*organizations.Options)) (*organizations.ListParentsOutput, error)
	ListTagsForResource(context.Context, *organizations.ListTagsForResourceInput, ...func(*organizations.Options)) (*organizations.ListTagsForResourceOutput, error)

	//sts
	AssumeRole(context.Context, *sts.AssumeRoleInput, ...func(*sts.Options)) (*sts.AssumeRoleOutput, error)
	GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
	GetFederationToken(context.Context, *sts.GetFederationTokenInput, ...func(*sts.Options)) (*sts.GetFederationTokenOutput, error)

	//Support
	CreateCase(context.Context, *support.CreateCaseInput, ...func(*support.Options)) (*support.CreateCaseOutput, error)
	DescribeCases(context.Context, *support.DescribeCasesInput, ...func(*support.Options)) (*support.DescribeCasesOutput, error)

	// S3
	ListBuckets(context.Context, *s3.ListBucketsInput, ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	DeleteBucket(context.Context, *s3.DeleteBucketInput, ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
	BatchDeleteBucketObjects(bucketName *string) error
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)

	// Route53
	ListHostedZones(context.Context, *route53.ListHostedZonesInput, ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error)
	DeleteHostedZone(context.Context, *route53.DeleteHostedZoneInput, ...func(*route53.Options)) (*route53.DeleteHostedZoneOutput, error)
	ListResourceRecordSets(context.Context, *route53.ListResourceRecordSetsInput, ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
	ChangeResourceRecordSets(context.Context, *route53.ChangeResourceRecordSetsInput, ...func(*route53.Options)) (*route53.ChangeResourceRecordSetsOutput, error)

	// Service Quota
	GetServiceQuota(context.Context, *servicequotas.GetServiceQuotaInput, ...func(*servicequotas.Options)) (*servicequotas.GetServiceQuotaOutput, error)
	RequestServiceQuotaIncrease(context.Context, *servicequotas.RequestServiceQuotaIncreaseInput, ...func(*servicequotas.Options)) (*servicequotas.RequestServiceQuotaIncreaseOutput, error)
	ListRequestedServiceQuotaChangeHistory(context.Context, *servicequotas.ListRequestedServiceQuotaChangeHistoryInput, ...func(*servicequotas.Options)) (*servicequotas.ListRequestedServiceQuotaChangeHistoryOutput, error)
	ListRequestedServiceQuotaChangeHistoryByQuota(context.Context, *servicequotas.ListRequestedServiceQuotaChangeHistoryByQuotaInput, ...func(*servicequotas.Options)) (*servicequotas.ListRequestedServiceQuotaChangeHistoryByQuotaOutput, error)
}

// awsClientV2 is the implementation of ClientV2
type awsClientV2 struct {
	acctClient          *account.Client
	ec2Client           *ec2.Client
	iamClient           *iam.Client
	orgClient           *organizations.Client
	stsClient           *sts.Client
	supportClient       *support.Client
	s3Client            *s3.Client
	route53client       *route53.Client
	serviceQuotasClient *servicequotas.Client
}

// NewAwsClientInputV2 contains the input parameters for creating a new AWS client
type NewAwsClientInputV2 struct {
	AwsCredsSecretIDKey     string
	AwsCredsSecretAccessKey string
	AwsToken                string
	AwsRegion               string
	SecretName              string
	NameSpace               string
}

// Account methods
func (c *awsClientV2) EnableRegion(ctx context.Context, input *account.EnableRegionInput, opts ...func(*account.Options)) (*account.EnableRegionOutput, error) {
	return c.acctClient.EnableRegion(ctx, input, opts...)
}

func (c *awsClientV2) GetRegionOptStatus(ctx context.Context, input *account.GetRegionOptStatusInput, opts ...func(*account.Options)) (*account.GetRegionOptStatusOutput, error) {
	return c.acctClient.GetRegionOptStatus(ctx, input, opts...)
}

// EC2 methods
func (c *awsClientV2) RunInstances(ctx context.Context, input *ec2.RunInstancesInput, opts ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
	return c.ec2Client.RunInstances(ctx, input, opts...)
}

func (c *awsClientV2) DescribeInstanceStatus(ctx context.Context, input *ec2.DescribeInstanceStatusInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error) {
	return c.ec2Client.DescribeInstanceStatus(ctx, input, opts...)
}

func (c *awsClientV2) TerminateInstances(ctx context.Context, input *ec2.TerminateInstancesInput, opts ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	return c.ec2Client.TerminateInstances(ctx, input, opts...)
}

func (c *awsClientV2) DescribeVolumes(ctx context.Context, input *ec2.DescribeVolumesInput, opts ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return c.ec2Client.DescribeVolumes(ctx, input, opts...)
}

func (c *awsClientV2) DeleteVolume(ctx context.Context, input *ec2.DeleteVolumeInput, opts ...func(*ec2.Options)) (*ec2.DeleteVolumeOutput, error) {
	return c.ec2Client.DeleteVolume(ctx, input, opts...)
}

func (c *awsClientV2) DescribeSnapshots(ctx context.Context, input *ec2.DescribeSnapshotsInput, opts ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return c.ec2Client.DescribeSnapshots(ctx, input, opts...)
}

func (c *awsClientV2) DeleteSnapshot(ctx context.Context, input *ec2.DeleteSnapshotInput, opts ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error) {
	return c.ec2Client.DeleteSnapshot(ctx, input, opts...)
}

func (c *awsClientV2) DescribeImages(ctx context.Context, input *ec2.DescribeImagesInput, opts ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return c.ec2Client.DescribeImages(ctx, input, opts...)
}

func (c *awsClientV2) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return c.ec2Client.DescribeInstances(ctx, input, opts...)
}

func (c *awsClientV2) DescribeInstanceTypes(ctx context.Context, input *ec2.DescribeInstanceTypesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
	return c.ec2Client.DescribeInstanceTypes(ctx, input, opts...)
}

func (c *awsClientV2) DescribeRegions(ctx context.Context, input *ec2.DescribeRegionsInput, opts ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	return c.ec2Client.DescribeRegions(ctx, input, opts...)
}

func (c *awsClientV2) DescribeVpcEndpointServiceConfigurations(ctx context.Context, input *ec2.DescribeVpcEndpointServiceConfigurationsInput, opts ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointServiceConfigurationsOutput, error) {
	return c.ec2Client.DescribeVpcEndpointServiceConfigurations(ctx, input, opts...)
}

func (c *awsClientV2) DeleteVpcEndpointServiceConfigurations(ctx context.Context, input *ec2.DeleteVpcEndpointServiceConfigurationsInput, opts ...func(*ec2.Options)) (*ec2.DeleteVpcEndpointServiceConfigurationsOutput, error) {
	return c.ec2Client.DeleteVpcEndpointServiceConfigurations(ctx, input, opts...)
}

func (c *awsClientV2) DescribeVpcs(ctx context.Context, input *ec2.DescribeVpcsInput, opts ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return c.ec2Client.DescribeVpcs(ctx, input, opts...)
}

func (c *awsClientV2) CreateVpc(ctx context.Context, input *ec2.CreateVpcInput, opts ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
	return c.ec2Client.CreateVpc(ctx, input, opts...)
}

func (c *awsClientV2) DeleteVpc(ctx context.Context, input *ec2.DeleteVpcInput, opts ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error) {
	return c.ec2Client.DeleteVpc(ctx, input, opts...)
}

func (c *awsClientV2) DescribeSubnets(ctx context.Context, input *ec2.DescribeSubnetsInput, opts ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return c.ec2Client.DescribeSubnets(ctx, input, opts...)
}

func (c *awsClientV2) CreateSubnet(ctx context.Context, input *ec2.CreateSubnetInput, opts ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
	return c.ec2Client.CreateSubnet(ctx, input, opts...)
}

func (c *awsClientV2) DeleteSubnet(ctx context.Context, input *ec2.DeleteSubnetInput, opts ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error) {
	return c.ec2Client.DeleteSubnet(ctx, input, opts...)
}

// IAM methods
func (c *awsClientV2) CreateAccessKey(ctx context.Context, input *iam.CreateAccessKeyInput, opts ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error) {
	return c.iamClient.CreateAccessKey(ctx, input, opts...)
}

func (c *awsClientV2) CreateUser(ctx context.Context, input *iam.CreateUserInput, opts ...func(*iam.Options)) (*iam.CreateUserOutput, error) {
	return c.iamClient.CreateUser(ctx, input, opts...)
}

func (c *awsClientV2) DeleteAccessKey(ctx context.Context, input *iam.DeleteAccessKeyInput, opts ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error) {
	return c.iamClient.DeleteAccessKey(ctx, input, opts...)
}

func (c *awsClientV2) DeleteUser(ctx context.Context, input *iam.DeleteUserInput, opts ...func(*iam.Options)) (*iam.DeleteUserOutput, error) {
	return c.iamClient.DeleteUser(ctx, input, opts...)
}

func (c *awsClientV2) DeleteUserPolicy(ctx context.Context, input *iam.DeleteUserPolicyInput, opts ...func(*iam.Options)) (*iam.DeleteUserPolicyOutput, error) {
	return c.iamClient.DeleteUserPolicy(ctx, input, opts...)
}

func (c *awsClientV2) GetUser(ctx context.Context, input *iam.GetUserInput, opts ...func(*iam.Options)) (*iam.GetUserOutput, error) {
	return c.iamClient.GetUser(ctx, input, opts...)
}

func (c *awsClientV2) ListUsers(ctx context.Context, input *iam.ListUsersInput, opts ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	return c.iamClient.ListUsers(ctx, input, opts...)
}

func (c *awsClientV2) ListUserTags(ctx context.Context, input *iam.ListUserTagsInput, opts ...func(*iam.Options)) (*iam.ListUserTagsOutput, error) {
	return c.iamClient.ListUserTags(ctx, input, opts...)
}

func (c *awsClientV2) ListAccessKeys(ctx context.Context, input *iam.ListAccessKeysInput, opts ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	return c.iamClient.ListAccessKeys(ctx, input, opts...)
}

func (c *awsClientV2) ListUserPolicies(ctx context.Context, input *iam.ListUserPoliciesInput, opts ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error) {
	return c.iamClient.ListUserPolicies(ctx, input, opts...)
}

func (c *awsClientV2) PutUserPolicy(ctx context.Context, input *iam.PutUserPolicyInput, opts ...func(*iam.Options)) (*iam.PutUserPolicyOutput, error) {
	return c.iamClient.PutUserPolicy(ctx, input, opts...)
}

func (c *awsClientV2) AttachUserPolicy(ctx context.Context, input *iam.AttachUserPolicyInput, opts ...func(*iam.Options)) (*iam.AttachUserPolicyOutput, error) {
	return c.iamClient.AttachUserPolicy(ctx, input, opts...)
}

func (c *awsClientV2) DetachUserPolicy(ctx context.Context, input *iam.DetachUserPolicyInput, opts ...func(*iam.Options)) (*iam.DetachUserPolicyOutput, error) {
	return c.iamClient.DetachUserPolicy(ctx, input, opts...)
}

func (c *awsClientV2) ListPolicies(ctx context.Context, input *iam.ListPoliciesInput, opts ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return c.iamClient.ListPolicies(ctx, input, opts...)
}

func (c *awsClientV2) ListAttachedUserPolicies(ctx context.Context, input *iam.ListAttachedUserPoliciesInput, opts ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	return c.iamClient.ListAttachedUserPolicies(ctx, input, opts...)
}

func (c *awsClientV2) CreatePolicy(ctx context.Context, input *iam.CreatePolicyInput, opts ...func(*iam.Options)) (*iam.CreatePolicyOutput, error) {
	return c.iamClient.CreatePolicy(ctx, input, opts...)
}

func (c *awsClientV2) DeletePolicy(ctx context.Context, input *iam.DeletePolicyInput, opts ...func(*iam.Options)) (*iam.DeletePolicyOutput, error) {
	return c.iamClient.DeletePolicy(ctx, input, opts...)
}

func (c *awsClientV2) DeletePolicyVersion(ctx context.Context, input *iam.DeletePolicyVersionInput, opts ...func(*iam.Options)) (*iam.DeletePolicyVersionOutput, error) {
	return c.iamClient.DeletePolicyVersion(ctx, input, opts...)
}

func (c *awsClientV2) GetPolicy(ctx context.Context, input *iam.GetPolicyInput, opts ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	return c.iamClient.GetPolicy(ctx, input, opts...)
}

func (c *awsClientV2) GetPolicyVersion(ctx context.Context, input *iam.GetPolicyVersionInput, opts ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	return c.iamClient.GetPolicyVersion(ctx, input, opts...)
}

func (c *awsClientV2) ListPolicyVersions(ctx context.Context, input *iam.ListPolicyVersionsInput, opts ...func(*iam.Options)) (*iam.ListPolicyVersionsOutput, error) {
	return c.iamClient.ListPolicyVersions(ctx, input, opts...)
}

func (c *awsClientV2) AttachRolePolicy(ctx context.Context, input *iam.AttachRolePolicyInput, opts ...func(*iam.Options)) (*iam.AttachRolePolicyOutput, error) {
	return c.iamClient.AttachRolePolicy(ctx, input, opts...)
}

func (c *awsClientV2) DetachRolePolicy(ctx context.Context, input *iam.DetachRolePolicyInput, opts ...func(*iam.Options)) (*iam.DetachRolePolicyOutput, error) {
	return c.iamClient.DetachRolePolicy(ctx, input, opts...)
}

func (c *awsClientV2) ListAttachedRolePolicies(ctx context.Context, input *iam.ListAttachedRolePoliciesInput, opts ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	return c.iamClient.ListAttachedRolePolicies(ctx, input, opts...)
}

func (c *awsClientV2) ListRolePolicies(ctx context.Context, input *iam.ListRolePoliciesInput, opts ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return c.iamClient.ListRolePolicies(ctx, input, opts...)
}

func (c *awsClientV2) DeleteRolePolicy(ctx context.Context, input *iam.DeleteRolePolicyInput, opts ...func(*iam.Options)) (*iam.DeleteRolePolicyOutput, error) {
	return c.iamClient.DeleteRolePolicy(ctx, input, opts...)
}

func (c *awsClientV2) CreateRole(ctx context.Context, input *iam.CreateRoleInput, opts ...func(*iam.Options)) (*iam.CreateRoleOutput, error) {
	return c.iamClient.CreateRole(ctx, input, opts...)
}

func (c *awsClientV2) GetRole(ctx context.Context, input *iam.GetRoleInput, opts ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	return c.iamClient.GetRole(ctx, input, opts...)
}

func (c *awsClientV2) DeleteRole(ctx context.Context, input *iam.DeleteRoleInput, opts ...func(*iam.Options)) (*iam.DeleteRoleOutput, error) {
	return c.iamClient.DeleteRole(ctx, input, opts...)
}

func (c *awsClientV2) ListRoles(ctx context.Context, input *iam.ListRolesInput, opts ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	return c.iamClient.ListRoles(ctx, input, opts...)
}

func (c *awsClientV2) PutRolePolicy(ctx context.Context, input *iam.PutRolePolicyInput, opts ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	return c.iamClient.PutRolePolicy(ctx, input, opts...)
}

// Organizations methods
func (c *awsClientV2) ListAccounts(ctx context.Context, input *organizations.ListAccountsInput, opts ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error) {
	return c.orgClient.ListAccounts(ctx, input, opts...)
}

func (c *awsClientV2) CreateAccount(ctx context.Context, input *organizations.CreateAccountInput, opts ...func(*organizations.Options)) (*organizations.CreateAccountOutput, error) {
	return c.orgClient.CreateAccount(ctx, input, opts...)
}

func (c *awsClientV2) DescribeCreateAccountStatus(ctx context.Context, input *organizations.DescribeCreateAccountStatusInput, opts ...func(*organizations.Options)) (*organizations.DescribeCreateAccountStatusOutput, error) {
	return c.orgClient.DescribeCreateAccountStatus(ctx, input, opts...)
}

func (c *awsClientV2) ListCreateAccountStatus(ctx context.Context, input *organizations.ListCreateAccountStatusInput, opts ...func(*organizations.Options)) (*organizations.ListCreateAccountStatusOutput, error) {
	return c.orgClient.ListCreateAccountStatus(ctx, input, opts...)
}

func (c *awsClientV2) MoveAccount(ctx context.Context, input *organizations.MoveAccountInput, opts ...func(*organizations.Options)) (*organizations.MoveAccountOutput, error) {
	return c.orgClient.MoveAccount(ctx, input, opts...)
}

func (c *awsClientV2) CreateOrganizationalUnit(ctx context.Context, input *organizations.CreateOrganizationalUnitInput, opts ...func(*organizations.Options)) (*organizations.CreateOrganizationalUnitOutput, error) {
	return c.orgClient.CreateOrganizationalUnit(ctx, input, opts...)
}

func (c *awsClientV2) ListOrganizationalUnitsForParent(ctx context.Context, input *organizations.ListOrganizationalUnitsForParentInput, opts ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error) {
	return c.orgClient.ListOrganizationalUnitsForParent(ctx, input, opts...)
}

func (c *awsClientV2) ListChildren(ctx context.Context, input *organizations.ListChildrenInput, opts ...func(*organizations.Options)) (*organizations.ListChildrenOutput, error) {
	return c.orgClient.ListChildren(ctx, input, opts...)
}

func (c *awsClientV2) TagResource(ctx context.Context, input *organizations.TagResourceInput, opts ...func(*organizations.Options)) (*organizations.TagResourceOutput, error) {
	return c.orgClient.TagResource(ctx, input, opts...)
}

func (c *awsClientV2) UntagResource(ctx context.Context, input *organizations.UntagResourceInput, opts ...func(*organizations.Options)) (*organizations.UntagResourceOutput, error) {
	return c.orgClient.UntagResource(ctx, input, opts...)
}

func (c *awsClientV2) ListParents(ctx context.Context, input *organizations.ListParentsInput, opts ...func(*organizations.Options)) (*organizations.ListParentsOutput, error) {
	return c.orgClient.ListParents(ctx, input, opts...)
}

func (c *awsClientV2) ListTagsForResource(ctx context.Context, input *organizations.ListTagsForResourceInput, opts ...func(*organizations.Options)) (*organizations.ListTagsForResourceOutput, error) {
	return c.orgClient.ListTagsForResource(ctx, input, opts...)
}

// STS methods
func (c *awsClientV2) AssumeRole(ctx context.Context, input *sts.AssumeRoleInput, opts ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	return c.stsClient.AssumeRole(ctx, input, opts...)
}

func (c *awsClientV2) GetCallerIdentity(ctx context.Context, input *sts.GetCallerIdentityInput, opts ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return c.stsClient.GetCallerIdentity(ctx, input, opts...)
}

func (c *awsClientV2) GetFederationToken(ctx context.Context, input *sts.GetFederationTokenInput, opts ...func(*sts.Options)) (*sts.GetFederationTokenOutput, error) {
	return c.stsClient.GetFederationToken(ctx, input, opts...)
}

// Support methods
func (c *awsClientV2) CreateCase(ctx context.Context, input *support.CreateCaseInput, opts ...func(*support.Options)) (*support.CreateCaseOutput, error) {
	return c.supportClient.CreateCase(ctx, input, opts...)
}

func (c *awsClientV2) DescribeCases(ctx context.Context, input *support.DescribeCasesInput, opts ...func(*support.Options)) (*support.DescribeCasesOutput, error) {
	return c.supportClient.DescribeCases(ctx, input, opts...)
}

// S3 methods
func (c *awsClientV2) ListBuckets(ctx context.Context, input *s3.ListBucketsInput, opts ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return c.s3Client.ListBuckets(ctx, input, opts...)
}

func (c *awsClientV2) DeleteBucket(ctx context.Context, input *s3.DeleteBucketInput, opts ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
	return c.s3Client.DeleteBucket(ctx, input, opts...)
}

func (c *awsClientV2) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return c.s3Client.ListObjectsV2(ctx, input, opts...)
}

func (c *awsClientV2) BatchDeleteBucketObjects(bucketName *string) error {
	// TODO: Implement batch delete for AWS SDK v2
	// This is a placeholder - the actual implementation would need to be updated
	return fmt.Errorf("BatchDeleteBucketObjects not implemented for AWS SDK v2")
}

// Route53 methods
func (c *awsClientV2) ListHostedZones(ctx context.Context, input *route53.ListHostedZonesInput, opts ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	return c.route53client.ListHostedZones(ctx, input, opts...)
}

func (c *awsClientV2) DeleteHostedZone(ctx context.Context, input *route53.DeleteHostedZoneInput, opts ...func(*route53.Options)) (*route53.DeleteHostedZoneOutput, error) {
	return c.route53client.DeleteHostedZone(ctx, input, opts...)
}

func (c *awsClientV2) ListResourceRecordSets(ctx context.Context, input *route53.ListResourceRecordSetsInput, opts ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	return c.route53client.ListResourceRecordSets(ctx, input, opts...)
}

func (c *awsClientV2) ChangeResourceRecordSets(ctx context.Context, input *route53.ChangeResourceRecordSetsInput, opts ...func(*route53.Options)) (*route53.ChangeResourceRecordSetsOutput, error) {
	return c.route53client.ChangeResourceRecordSets(ctx, input, opts...)
}

// Service Quota methods
func (c *awsClientV2) GetServiceQuota(ctx context.Context, input *servicequotas.GetServiceQuotaInput, opts ...func(*servicequotas.Options)) (*servicequotas.GetServiceQuotaOutput, error) {
	return c.serviceQuotasClient.GetServiceQuota(ctx, input, opts...)
}

func (c *awsClientV2) RequestServiceQuotaIncrease(ctx context.Context, input *servicequotas.RequestServiceQuotaIncreaseInput, opts ...func(*servicequotas.Options)) (*servicequotas.RequestServiceQuotaIncreaseOutput, error) {
	return c.serviceQuotasClient.RequestServiceQuotaIncrease(ctx, input, opts...)
}

func (c *awsClientV2) ListRequestedServiceQuotaChangeHistory(ctx context.Context, input *servicequotas.ListRequestedServiceQuotaChangeHistoryInput, opts ...func(*servicequotas.Options)) (*servicequotas.ListRequestedServiceQuotaChangeHistoryOutput, error) {
	return c.serviceQuotasClient.ListRequestedServiceQuotaChangeHistory(ctx, input, opts...)
}

func (c *awsClientV2) ListRequestedServiceQuotaChangeHistoryByQuota(ctx context.Context, input *servicequotas.ListRequestedServiceQuotaChangeHistoryByQuotaInput, opts ...func(*servicequotas.Options)) (*servicequotas.ListRequestedServiceQuotaChangeHistoryByQuotaOutput, error) {
	return c.serviceQuotasClient.ListRequestedServiceQuotaChangeHistoryByQuota(ctx, input, opts...)
}

// newClientV2 creates a new AWS SDK v2 client
func newClientV2(controllerName, awsAccessID, awsAccessSecret, token, region string) (ClientV2, error) {
	var cfg aws.Config
	var err error

	if awsAccessID != "" && awsAccessSecret != "" {
		// Use provided credentials
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awsAccessID, awsAccessSecret, token)),
		)
	} else {
		// Use default credentials (IAM role, environment variables, etc.)
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// TODO: Add metrics middleware for AWS SDK v2
	// The metrics middleware needs to be implemented for AWS SDK v2

	return &awsClientV2{
		acctClient:          account.NewFromConfig(cfg),
		ec2Client:           ec2.NewFromConfig(cfg),
		iamClient:           iam.NewFromConfig(cfg),
		orgClient:           organizations.NewFromConfig(cfg),
		stsClient:           sts.NewFromConfig(cfg),
		supportClient:       support.NewFromConfig(cfg),
		s3Client:            s3.NewFromConfig(cfg),
		route53client:       route53.NewFromConfig(cfg),
		serviceQuotasClient: servicequotas.NewFromConfig(cfg),
	}, nil
}

// IBuilderV2 interface for building AWS SDK v2 clients
type IBuilderV2 interface {
	GetClientV2(controllerName string, kubeClient kubeclientpkg.Client, input NewAwsClientInputV2) (ClientV2, error)
}

// BuilderV2 implements IBuilderV2
type BuilderV2 struct{}

// GetClientV2 creates a new AWS SDK v2 client
func (rp *BuilderV2) GetClientV2(controllerName string, kubeClient kubeclientpkg.Client, input NewAwsClientInputV2) (ClientV2, error) {
	var awsAccessID, awsAccessSecret, token string

	if input.SecretName != "" && input.NameSpace != "" {
		// Get credentials from Kubernetes secret
		secret := &corev1.Secret{}
		err := kubeClient.Get(context.TODO(), types.NamespacedName{Name: input.SecretName, Namespace: input.NameSpace}, secret)
		if err != nil {
			return nil, fmt.Errorf("failed to get secret %s in namespace %s: %w", input.SecretName, input.NameSpace, err)
		}

		awsAccessID = string(secret.Data[awsCredsSecretIDKeyV2])
		awsAccessSecret = string(secret.Data[awsCredsSecretAccessKeyV2])
	} else {
		// Use provided credentials
		awsAccessID = input.AwsCredsSecretIDKey
		awsAccessSecret = input.AwsCredsSecretAccessKey
		token = input.AwsToken
	}

	return newClientV2(controllerName, awsAccessID, awsAccessSecret, token, input.AwsRegion)
}
