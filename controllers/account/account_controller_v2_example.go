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

package account

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/go-logr/logr"
	awsv1alpha1 "github.com/openshift/aws-account-operator/api/v1alpha1"
	"github.com/openshift/aws-account-operator/config"
	"github.com/openshift/aws-account-operator/pkg/awsclient"
	stsclient "github.com/openshift/aws-account-operator/pkg/awsclient/sts"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AccountReconcilerV2Example demonstrates how to use AWS SDK v2
type AccountReconcilerV2Example struct {
	client.Client
	Scheme           *runtime.Scheme
	awsClientBuilder awsclient.IBuilderV2
	shardName        string
}

//+kubebuilder:rbac:groups=aws.managed.openshift.io,resources=accounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aws.managed.openshift.io,resources=accounts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aws.managed.openshift.io,resources=accounts/finalizers,verbs=update

// Reconcile demonstrates AWS SDK v2 usage
func (r *AccountReconcilerV2Example) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := logr.FromContextOrDiscard(ctx)
	reqLogger.Info("Reconciling Account with AWS SDK v2")

	// Fetch the Account instance
	account := &awsv1alpha1.Account{}
	err := r.Get(ctx, request.NamespacedName, account)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Example: Create AWS client using v2
	awsClient, err := r.awsClientBuilder.GetClientV2("account", r.Client, awsclient.NewAwsClientInput{
		AwsRegion: config.GetDefaultRegion(),
		// Add credentials as needed
	})
	if err != nil {
		reqLogger.Error(err, "Failed to create AWS client")
		return reconcile.Result{}, err
	}

	// Example: Create an account using AWS SDK v2
	if account.Status.State == "" {
		return r.createAccountV2(ctx, reqLogger, account, awsClient)
	}

	return reconcile.Result{}, nil
}

// createAccountV2 demonstrates creating an account with AWS SDK v2
func (r *AccountReconcilerV2Example) createAccountV2(ctx context.Context, reqLogger logr.Logger, account *awsv1alpha1.Account, awsClient awsclient.ClientV2) (reconcile.Result, error) {
	reqLogger.Info("Creating account with AWS SDK v2")

	// Example: Create account using Organizations API v2
	createAccountInput := &organizations.CreateAccountInput{
		AccountName: aws.String(account.Name),
		Email:       aws.String(fmt.Sprintf("%s@example.com", account.Name)),
	}

	result, err := awsClient.CreateAccount(ctx, createAccountInput)
	if err != nil {
		reqLogger.Error(err, "Failed to create account")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Account creation initiated", "requestId", *result.CreateAccountStatus.Id)

	// Example: Describe account creation status
	describeInput := &organizations.DescribeCreateAccountStatusInput{
		CreateAccountRequestId: result.CreateAccountStatus.Id,
	}

	statusResult, err := awsClient.DescribeCreateAccountStatus(ctx, describeInput)
	if err != nil {
		reqLogger.Error(err, "Failed to describe account status")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Account status", "state", statusResult.CreateAccountStatus.State)

	// Example: Assume role with AWS SDK v2
	if account.Spec.AwsAccountID != "" {
		assumedClient, creds, err := stsclient.AssumeRoleAndCreateClientV2(
			reqLogger, r.awsClientBuilder, account, r.Client, awsClient,
			config.GetDefaultRegion(), "OrganizationAccountAccessRole", "")
		if err != nil {
			reqLogger.Error(err, "Failed to assume role")
			return reconcile.Result{}, err
		}

		// Example: Use assumed role to create IAM user
		return r.createIAMUserV2(ctx, reqLogger, assumedClient, creds)
	}

	return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}

// createIAMUserV2 demonstrates creating an IAM user with AWS SDK v2
func (r *AccountReconcilerV2Example) createIAMUserV2(ctx context.Context, reqLogger logr.Logger, awsClient awsclient.ClientV2, creds *sts.AssumeRoleOutput) (reconcile.Result, error) {
	reqLogger.Info("Creating IAM user with AWS SDK v2")

	// Example: Create IAM user
	createUserInput := &iam.CreateUserInput{
		UserName: aws.String("example-user"),
	}

	userResult, err := awsClient.CreateUser(ctx, createUserInput)
	if err != nil {
		reqLogger.Error(err, "Failed to create IAM user")
		return reconcile.Result{}, err
	}

	reqLogger.Info("IAM user created", "userName", *userResult.User.UserName)

	// Example: Create access key
	createAccessKeyInput := &iam.CreateAccessKeyInput{
		UserName: userResult.User.UserName,
	}

	accessKeyResult, err := awsClient.CreateAccessKey(ctx, createAccessKeyInput)
	if err != nil {
		reqLogger.Error(err, "Failed to create access key")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Access key created", "accessKeyId", *accessKeyResult.AccessKey.AccessKeyId)

	// Example: Run EC2 instance
	runInstancesInput := &ec2.RunInstancesInput{
		ImageId:      aws.String("ami-12345678"),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		InstanceType: "t2.micro",
	}

	instanceResult, err := awsClient.RunInstances(ctx, runInstancesInput)
	if err != nil {
		reqLogger.Error(err, "Failed to run instance")
		return reconcile.Result{}, err
	}

	reqLogger.Info("EC2 instance launched", "instanceId", *instanceResult.Instances[0].InstanceId)

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the manager
func (r *AccountReconcilerV2Example) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&awsv1alpha1.Account{}).
		Complete(r)
}
