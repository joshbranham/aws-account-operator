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
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	orgtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
	stsclient "github.com/openshift/aws-account-operator/pkg/awsclient/sts"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	awsv1alpha1 "github.com/openshift/aws-account-operator/api/v1alpha1"
	"github.com/openshift/aws-account-operator/config"
	"github.com/openshift/aws-account-operator/pkg/awsclient"
	"github.com/openshift/aws-account-operator/pkg/totalaccountwatcher"
	"github.com/openshift/aws-account-operator/pkg/utils"
)

var logV2 = logf.Log.WithName("controller_account_v2")
var AssumeRoleAndCreateClientV2 = stsclient.AssumeRoleAndCreateClientV2

const (
	// createPendTimeV2 is the maximum time we allow an Account to sit in Creating state before we
	// time out and set it to Failed.
	createPendTimeV2 = utils.WaitTime * time.Minute
	// regionInitTimeV2 is the maximum time we allow an account CR to be in the InitializingRegions
	// state. This is based on async region init taking a theoretical maximum of WaitTime * 2
	// minutes plus a handful of AWS API calls (see asyncRegionInit).
	regionInitTimeV2 = (time.Minute * utils.WaitTime * time.Duration(2)) + time.Minute
	// awsAccountInitRequeueDurationV2 is the duration we want to wait for the next
	// reconcile loop after hitting an OptInRequired-error during region initialization.
	awsAccountInitRequeueDurationV2 = 1 * time.Minute

	// AccountPendingV2 indicates an account is pending
	AccountPendingV2 = "Pending"
	// AccountCreatingV2 indicates an account is being created
	AccountCreatingV2 = "Creating"
	// AccountFailedV2 indicates account creation has failed
	AccountFailedV2 = "Failed"
	// AccountInitializingRegionsV2 indicates we've kicked off the process of creating and terminating
	// instances in all supported regions
	AccountInitializingRegionsV2 = "InitializingRegions"
	// AccountReadyV2 indicates account creation is ready
	AccountReadyV2 = "Ready"
	// AccountPendingVerificationV2 indicates verification (of AWS limits and Enterprise Support) is pending
	AccountPendingVerificationV2 = "PendingVerification"
	// AccountOptingInRegionsV2 indicates region enablement for supported Opt-In regions is in progress
	AccountOptingInRegionsV2 = "OptingInRegions"
	// AccountOptInRegionEnabledV2 indicates that supported Opt-In regions have been enabled
	AccountOptInRegionEnabledV2    = "OptInRegionsEnabled"
	standardAdminAccessArnPrefixV2 = "arn:aws:iam"
	adminAccessArnSuffixV2         = "::aws:policy/AdministratorAccess"
	iamUserNameUHCV2               = "osdManagedAdmin"

	controllerNameV2 = "account_v2"

	// number of service quota requests we are allowed to open concurrently in AWS
	MaxOpenQuotaRequestsV2 = 20

	// MaxOptInRegionRequestV2 maximum number of regions that AWS allows to be concurrently enabled
	MaxOptInRegionRequestV2 = 6
	// MaxAccountRegionEnablementV2 maximum number of AWS accounts allowed to enable all regions simultaneously
	MaxAccountRegionEnablementV2 = 9
)

// AccountReconcilerV2 reconciles a Account object using AWS SDK v2
type AccountReconcilerV2 struct {
	client.Client
	Scheme           *runtime.Scheme
	awsClientBuilder awsclient.IBuilderV2
	shardName        string
}

//+kubebuilder:rbac:groups=aws.managed.openshift.io,resources=accounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aws.managed.openshift.io,resources=accounts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aws.managed.openshift.io,resources=accounts/finalizers,verbs=update

// Reconcile reads that state of the cluster for a Account object and makes changes based on the state read
// and what is in the Account.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *AccountReconcilerV2) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.WithValues("Controller", controllerName, "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	// Fetch the Account instance
	currentAcctInstance := &awsv1alpha1.Account{}
	err := r.Get(ctx, request.NamespacedName, currentAcctInstance)

	if err != nil {
		if k8serr.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	configMap, err := utils.GetOperatorConfigMap(r.Client)
	if err != nil {
		log.Error(err, "Failed retrieving configmap")
		return reconcile.Result{}, err
	}

	isOptInRegionFeatureEnabled, err := utils.GetFeatureFlagValue(configMap, "feature.opt_in_regions")
	if err != nil {
		reqLogger.Info("Could not retrieve feature flag 'feature.opt_in_regions' - region Opt-In is disabled")
		isOptInRegionFeatureEnabled = false
	}
	reqLogger.Info("Is feature.opt_in_regions enabled?", "enabled", isOptInRegionFeatureEnabled)

	optInRegions, ok := configMap.Data["opt-in-regions"]
	if !ok {
		reqLogger.Info("Could not retrieve opt-in-regions from configMap")
	}

	awsRegion := config.GetDefaultRegion()
	// We expect this secret to exist in the same namespace Account CR's are created
	awsSetupClient, err := r.awsClientBuilder.GetClientV2(controllerName, r.Client, awsclient.NewAwsClientInputV2{
		SecretName: utils.AwsSecretName,
		NameSpace:  awsv1alpha1.AccountCrNamespace,
		AwsRegion:  awsRegion,
	})
	if err != nil {
		reqLogger.Error(err, "failed building operator AWS client")
		return reconcile.Result{}, err
	}

	// Add finalizer to non-sts account cr
	if !currentAcctInstance.Spec.ManualSTSMode {
		err := r.addFinalizer(ctx, reqLogger, currentAcctInstance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	if currentAcctInstance.IsPendingDeletion() {
		if currentAcctInstance.Spec.ManualSTSMode {
			// if the account is STS, we don't need to do any additional cleanup aside from
			// removing the finalizer and exiting.
			err := r.removeFinalizer(ctx, currentAcctInstance, awsv1alpha1.AccountFinalizer)
			if err != nil {
				reqLogger.Error(err, "Failed removing account finalizer")
			}
			return reconcile.Result{}, err
		}

		var awsClient awsclient.ClientV2
		if currentAcctInstance.IsBYOC() {
			roleToAssume := currentAcctInstance.GetAssumeRole()
			awsClient, _, err = stsclient.HandleRoleAssumptionV2(reqLogger, r.awsClientBuilder, currentAcctInstance, r.Client, awsSetupClient, "", roleToAssume, "")
			if err != nil {
				reqLogger.Error(err, "failed building BYOC client from assume_role")
				_, err = r.handleAWSClientErrorV2(ctx, reqLogger, currentAcctInstance, err)
				// TODO: Update error handling for AWS SDK v2 error types
				return reconcile.Result{}, err
			}
		} else {
			awsClient, _, err = stsclient.HandleRoleAssumptionV2(reqLogger, r.awsClientBuilder, currentAcctInstance, r.Client, awsSetupClient, "", awsv1alpha1.AccountOperatorIAMRole, "")
			if err != nil {
				reqLogger.Error(err, "failed building AWS client from assume_role")
				return r.handleAWSClientErrorV2(ctx, reqLogger, currentAcctInstance, err)
			}
		}
		r.finalizeAccountV2(ctx, reqLogger, awsClient, currentAcctInstance)

		// Remove finalizer if account CR is non STS. For CCS accounts, the accountclaim controller will delete the account CR
		// when the accountClaim CR is deleted as its set as the owner reference.
		if currentAcctInstance.IsNonSTSPendingDeletionWithFinalizer() {
			reqLogger.Info("removing account finalizer")
			err = r.removeFinalizer(ctx, currentAcctInstance, awsv1alpha1.AccountFinalizer)
			if err != nil {
				reqLogger.Error(err, "failed removing account finalizer")
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	// Log accounts that have failed and don't attempt to reconcile them
	if currentAcctInstance.IsFailed() {
		reqLogger.Info(fmt.Sprintf("Account %s is failed. Ignoring.", currentAcctInstance.Name))
		return reconcile.Result{}, nil
	}

	// Detect accounts for which we kicked off asynchronous region initialization
	if currentAcctInstance.IsInitializingRegions() {
		return r.handleAccountInitializingRegionsV2(ctx, reqLogger, currentAcctInstance)
	}

	// If the account is BYOC, needs some different set up
	if newBYOCAccountV2(currentAcctInstance) {
		var result reconcile.Result
		var initErr error

		result, initErr = r.initializeNewCCSAccountV2(ctx, reqLogger, currentAcctInstance)
		if initErr != nil {
			_, stateErr := r.setAccountFailedV2(ctx, reqLogger, currentAcctInstance, awsv1alpha1.AccountCreationFailed, initErr.Error(), "Failed to initialize new CCS account", AccountFailed)
			if stateErr != nil {
				reqLogger.Error(stateErr, "failed setting account state", "desiredState", AccountFailed)
			}
			reqLogger.Error(initErr, "failed initializing new CCS account")
			return result, initErr
		}
		utils.SetAccountStatus(currentAcctInstance, AccountCreating, awsv1alpha1.AccountCreating, AccountCreating)
		updateErr := r.statusUpdate(ctx, currentAcctInstance)
		if updateErr != nil {
			reqLogger.Info("failed updating account state, retrying", "desired state", AccountCreating)
			return reconcile.Result{}, updateErr
		}
	} else {
		// Normal account creation

		// Test PendingVerification state creating support case and checking for case status
		if currentAcctInstance.IsPendingVerification() {
			return r.HandleNonCCSPendingVerificationV2(ctx, reqLogger, currentAcctInstance, awsSetupClient)
		}

		// Update account Status.Claimed to true if the account is ready and the claim link is not empty
		if currentAcctInstance.IsReadyUnclaimedAndHasClaimLink() {
			return reconcile.Result{}, ClaimAccountV2(r, currentAcctInstance)
		}

		// see if in creating for longer than default wait time
		if currentAcctInstance.IsCreating() && utils.CreationConditionOlderThan(*currentAcctInstance, createPendTime) {
			errMsg := fmt.Sprintf("Creation pending for longer than %d minutes", utils.WaitTime)
			_, stateErr := r.setAccountFailedV2(ctx, reqLogger, currentAcctInstance, awsv1alpha1.AccountCreationFailed, "CreationTimeout", errMsg, AccountFailed)
			if stateErr != nil {
				reqLogger.Error(stateErr, "failed setting account state", "desiredState", AccountFailed)
				return reconcile.Result{}, stateErr
			}
			return reconcile.Result{}, errors.New(errMsg)
		}

		if currentAcctInstance.IsUnclaimedAndHasNoState() {
			if !currentAcctInstance.HasAwsAccountID() {
				// before doing anything make sure we are not over the limit if we are just error
				if !totalaccountwatcher.TotalAccountWatcher.AccountsCanBeCreated() {
					// fedramp clusters are all CCS, so the account limit is irrelevant there
					if !config.IsFedramp() {
						reqLogger.Info("AWS Account limit reached. This does not always indicate a problem, it's a limit we enforce in the configmap to prevent runaway account creation")
						// We don't expect the limit to change very frequently, so wait a while before requeueing to avoid hot lopping.
						return reconcile.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Minute}, nil
					}
				}

				if err := r.nonCCSAssignAccountIDV2(ctx, reqLogger, currentAcctInstance, awsSetupClient); err != nil {
					return reconcile.Result{}, err
				}
			} else {
				// set state creating if the account was already created
				utils.SetAccountStatus(currentAcctInstance, "AWS account already created", awsv1alpha1.AccountCreating, AccountCreating)
				err = r.statusUpdate(ctx, currentAcctInstance)

				if err != nil {
					return reconcile.Result{}, err
				}
			}
		}
	}

	// Handles account region enablement for non-BYOC accounts
	if (currentAcctInstance.ReadyForRegionEnablement() || currentAcctInstance.IsEnablingOptInRegions()) && isOptInRegionFeatureEnabled && optInRegions != "" {
		return r.handleOptInRegionEnablementV2(ctx, reqLogger, currentAcctInstance, awsSetupClient, optInRegions)
	}

	// Get the owner of the Red Hat amis from the configmap
	amiOwner, ok := configMap.Data["ami-owner"]
	if !ok {
		err = awsv1alpha1.ErrInvalidConfigMap
		return reconcile.Result{}, err
	}
	if err != nil {
		reqLogger.Error(err, "failed getting ami-owner from configmap data")
		return reconcile.Result{}, err
	}

	// Account init for both BYOC and Non-BYOC
	if currentAcctInstance.ReadyForInitialization() {
		reqLogger.Info("initializing account", "awsAccountID", currentAcctInstance.Spec.AwsAccountID)

		var creds *sts.AssumeRoleOutput

		// STS mode doesn't need IAM user init, so just get the creds necessary, init regions, and exit
		if currentAcctInstance.Spec.ManualSTSMode {
			accountClaim, acctClaimErr := r.getAccountClaim(ctx, currentAcctInstance)
			if acctClaimErr != nil {
				reqLogger.Error(acctClaimErr, "unable to get accountclaim for sts account")
				utils.SetAccountClaimStatus(
					accountClaim,
					"Failed to get AccountClaim for CSS account",
					"FailedRetrievingAccountClaim",
					awsv1alpha1.ClientError,
					awsv1alpha1.ClaimStatusError,
				)
				err := r.Client.Status().Update(ctx, accountClaim)
				if err != nil {
					reqLogger.Error(err, "failed to update accountclaim status")
				}
				return reconcile.Result{}, acctClaimErr
			}

			_, creds, err = r.getSTSClientV2(ctx, reqLogger, accountClaim, awsSetupClient)
			if err != nil {
				reqLogger.Error(err, "error getting sts client to initialize regions")
				return reconcile.Result{}, err
			}
		} else {
			// Set IAMUserIDLabel if not there, and requeue
			if !utils.AccountCRHasIAMUserIDLabel(currentAcctInstance) {
				utils.AddLabels(
					currentAcctInstance,
					utils.GenerateLabel(
						awsv1alpha1.IAMUserIDLabel,
						utils.GenerateShortUID(),
					),
				)
				return reconcile.Result{Requeue: true}, r.Update(ctx, currentAcctInstance)
			}

			_, newCredentials, err := r.handleIAMUserCreationV2(ctx, reqLogger, currentAcctInstance, awsSetupClient, request.Namespace)
			if err != nil {
				reqLogger.Error(err, "Error during IAM user creation")
				return reconcile.Result{}, err
			}
			creds = newCredentials
		}

		err = r.initializeRegionsV2(ctx, reqLogger, currentAcctInstance, creds, amiOwner)

		if isAwsOptInErrorV2(err) {
			reqLogger.Info("Aws Account not ready yet, requeuing.")
			return reconcile.Result{
				RequeueAfter: awsAccountInitRequeueDurationV2,
			}, nil
		}

		if err != nil {
			reqLogger.Error(err, "failed to initialize regions")
			return reconcile.Result{}, err
		}

		// Set account to ready
		utils.SetAccountStatus(currentAcctInstance, AccountReady, awsv1alpha1.AccountReady, AccountReady)
		err = r.statusUpdate(ctx, currentAcctInstance)
		if err != nil {
			reqLogger.Error(err, "failed updating account status")
			return reconcile.Result{}, err
		}

		reqLogger.Info("Account initialization completed successfully")
	}

	return reconcile.Result{}, nil
}

// Helper functions for AWS SDK v2 migration

func (r *AccountReconcilerV2) addFinalizer(ctx context.Context, reqLogger logr.Logger, account *awsv1alpha1.Account) error {
	if !utils.Contains(account.GetFinalizers(), awsv1alpha1.AccountFinalizer) {
		reqLogger.Info("Adding Finalizer for the Account")
		account.SetFinalizers(append(account.GetFinalizers(), awsv1alpha1.AccountFinalizer))

		// Update CR
		err := r.Update(ctx, account)
		if err != nil {
			reqLogger.Error(err, "Failed to update Account with finalizer")
			return err
		}
	}
	return nil
}

// Function to remove finalizer
func (r *AccountReconcilerV2) removeFinalizer(ctx context.Context, account *awsv1alpha1.Account, finalizerName string) error {
	account.SetFinalizers(utils.Remove(account.GetFinalizers(), finalizerName))
	err := r.Update(ctx, account)
	if err != nil {
		return err
	}
	return nil
}

func (r *AccountReconcilerV2) handleAWSClientErrorV2(ctx context.Context, reqLogger logr.Logger, currentAcctInstance *awsv1alpha1.Account, err error) (reconcile.Result, error) {
	// Get custom failure reason to update account status
	reason := ""
	var awsErr *smithy.OperationError
	if errors.As(err, &awsErr) {
		reason = awsErr.Err.Error()
	}
	errMsg := fmt.Sprintf("Failed to create STS Credentials for account ID %s: %s", currentAcctInstance.Spec.AwsAccountID, err)
	_, stateErr := r.setAccountFailedV2(ctx, reqLogger, currentAcctInstance, awsv1alpha1.AccountClientError, reason, errMsg, AccountFailed)
	if stateErr != nil {
		reqLogger.Error(stateErr, "failed setting account state", "desiredState", AccountFailed)
	}

	return reconcile.Result{}, err
}

func (r *AccountReconcilerV2) handleAccountInitializingRegionsV2(ctx context.Context, reqLogger logr.Logger, currentAcctInstance *awsv1alpha1.Account) (reconcile.Result, error) {
	// TODO: Implement region initialization logic for AWS SDK v2
	reqLogger.Info("Handling account initializing regions")
	return reconcile.Result{}, nil
}

func (r *AccountReconcilerV2) initializeNewCCSAccountV2(ctx context.Context, reqLogger logr.Logger, currentAcctInstance *awsv1alpha1.Account) (reconcile.Result, error) {
	// TODO: Implement CCS account initialization for AWS SDK v2
	reqLogger.Info("Initializing new CCS account")
	return reconcile.Result{}, nil
}

func (r *AccountReconcilerV2) setAccountFailedV2(ctx context.Context, reqLogger logr.Logger, account *awsv1alpha1.Account, ctype awsv1alpha1.AccountConditionType, reason string, message string, state string) (reconcile.Result, error) {
	utils.SetAccountStatus(account, state, ctype, message)
	err := r.statusUpdate(ctx, account)
	if err != nil {
		reqLogger.Error(err, "failed updating account status")
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *AccountReconcilerV2) HandleNonCCSPendingVerificationV2(ctx context.Context, reqLogger logr.Logger, currentAcctInstance *awsv1alpha1.Account, awsSetupClient awsclient.ClientV2) (reconcile.Result, error) {
	// TODO: Implement pending verification logic for AWS SDK v2
	reqLogger.Info("Handling non-CCS pending verification")
	return reconcile.Result{}, nil
}

func (r *AccountReconcilerV2) nonCCSAssignAccountIDV2(ctx context.Context, reqLogger logr.Logger, currentAcctInstance *awsv1alpha1.Account, awsSetupClient awsclient.ClientV2) error {
	// Build Aws Account
	var awsAccountID string

	switch utils.DetectDevMode {
	case utils.DevModeProduction:
		var err error
		awsAccountID, err = r.BuildAccountV2(ctx, reqLogger, awsSetupClient, currentAcctInstance)
		if err != nil {
			return err
		}
	default:
		reqLogger.Info("Running in development mode, skipping account creation")
		awsAccountID = "123456789012"
	}

	// set state creating if the account was able to create
	utils.SetAccountStatus(currentAcctInstance, AccountCreating, awsv1alpha1.AccountCreating, AccountCreating)
	err := r.statusUpdate(ctx, currentAcctInstance)

	if err != nil {
		return err
	}

	if utils.DetectDevMode != utils.DevModeProduction {
		reqLogger.Info("Running in development mode, manually creating a case ID number: 11111111")
		currentAcctInstance.Status.SupportCaseID = "11111111"
	}

	// update account cr with awsAccountID from aws
	currentAcctInstance.Spec.AwsAccountID = awsAccountID

	// tag account with hive shard name
	err = r.TagAccountV2(ctx, awsSetupClient, awsAccountID, r.shardName)
	if err != nil {
		reqLogger.Info("Unable to tag aws account.", "account", currentAcctInstance.Name, "AWSAccountID", awsAccountID, "Error", err.Error())
	}

	return r.accountSpecUpdate(ctx, reqLogger, currentAcctInstance)
}

func (r *AccountReconcilerV2) TagAccountV2(ctx context.Context, awsSetupClient awsclient.ClientV2, awsAccountID string, shardName string) error {
	inputTag := &organizations.TagResourceInput{
		ResourceId: aws.String(awsAccountID),
		Tags: []orgtypes.Tag{
			{
				Key:   aws.String("owner"),
				Value: aws.String(shardName),
			},
		},
	}

	_, err := awsSetupClient.TagResource(ctx, inputTag)
	if err != nil {
		return err
	}

	return nil
}

func (r *AccountReconcilerV2) BuildAccountV2(ctx context.Context, reqLogger logr.Logger, awsClient awsclient.ClientV2, account *awsv1alpha1.Account) (string, error) {
	reqLogger.Info("Creating Account")

	email := formatAccountEmail(account.Name)
	orgOutput, orgErr := r.CreateAccountV2(ctx, reqLogger, awsClient, account.Name, email)
	// If it was an api or a limit issue don't modify account and exit if anything else set to failed
	if orgErr != nil {
		switch orgErr {
		case awsv1alpha1.ErrAwsFailedCreateAccount:
			utils.SetAccountStatus(account, "Failed to create AWS Account", awsv1alpha1.AccountCreationFailed, AccountFailed)
			err := r.statusUpdate(ctx, account)
			if err != nil {
				return "", err
			}

			reqLogger.Error(awsv1alpha1.ErrAwsFailedCreateAccount, "Failed to create AWS Account")
			return "", orgErr

		case awsv1alpha1.ErrAwsAccountLimitExceeded:
			reqLogger.Error(orgErr, "Failed to create AWS Account limit reached")
			return "", orgErr

		default:
			reqLogger.Error(orgErr, "Failed to create AWS Account nonfatal error")
			return "", orgErr
		}
	}

	accountObjectKey := client.ObjectKeyFromObject(account)
	err := r.Get(ctx, accountObjectKey, account)
	if err != nil {
		reqLogger.Error(err, "Unable to get updated Account object after status update")
	}

	reqLogger.Info("account created successfully")

	return *orgOutput.CreateAccountStatus.AccountId, nil
}

// CreateAccountV2 creates an AWS account for the specified accountName and accountEmail in the organization
func (r *AccountReconcilerV2) CreateAccountV2(ctx context.Context, reqLogger logr.Logger, client awsclient.ClientV2, accountName, accountEmail string) (*organizations.DescribeCreateAccountStatusOutput, error) {

	createInput := organizations.CreateAccountInput{
		AccountName: aws.String(accountName),
		Email:       aws.String(accountEmail),
	}

	createOutput, err := client.CreateAccount(ctx, &createInput)
	if err != nil {
		errMsg := "Error creating account"
		var returnErr error
		var awsErr *smithy.OperationError
		if errors.As(err, &awsErr) {
			switch awsErr.Err.Error() {
			case "ConcurrentModificationException":
				returnErr = awsv1alpha1.ErrAwsConcurrentModification
			case "ConstraintViolationException":
				returnErr = awsv1alpha1.ErrAwsAccountLimitExceeded
			case "ServiceException":
				returnErr = awsv1alpha1.ErrAwsInternalFailure
			case "TooManyRequestsException":
				returnErr = awsv1alpha1.ErrAwsTooManyRequests
			default:
				returnErr = awsv1alpha1.ErrAwsFailedCreateAccount
			}
		}
		utils.LogAwsError(reqLogger, errMsg, returnErr, err)
		return &organizations.DescribeCreateAccountStatusOutput{}, returnErr
	}

	describeStatusInput := organizations.DescribeCreateAccountStatusInput{
		CreateAccountRequestId: createOutput.CreateAccountStatus.Id,
	}

	var accountStatus *organizations.DescribeCreateAccountStatusOutput
	for {
		status, err := client.DescribeCreateAccountStatus(ctx, &describeStatusInput)
		if err != nil {
			return &organizations.DescribeCreateAccountStatusOutput{}, err
		}

		accountStatus = status
		createStatus := status.CreateAccountStatus.State

		if createStatus == "FAILED" {
			var returnErr error
			switch status.CreateAccountStatus.FailureReason {
			case "ACCOUNT_LIMIT_EXCEEDED":
				returnErr = awsv1alpha1.ErrAwsAccountLimitExceeded
			case "INTERNAL_FAILURE":
				returnErr = awsv1alpha1.ErrAwsInternalFailure
			default:
				returnErr = awsv1alpha1.ErrAwsFailedCreateAccount
			}

			return &organizations.DescribeCreateAccountStatusOutput{}, returnErr
		}

		if createStatus != "IN_PROGRESS" {
			break
		}

		time.Sleep(10 * time.Second)
	}

	return accountStatus, nil
}

func (r *AccountReconcilerV2) accountSpecUpdate(ctx context.Context, reqLogger logr.Logger, account *awsv1alpha1.Account) error {
	err := r.Update(ctx, account)
	if err != nil {
		reqLogger.Error(err, "Error updating Account CR")
		return err
	}
	return nil
}

func (r *AccountReconcilerV2) handleOptInRegionEnablementV2(ctx context.Context, reqLogger logr.Logger, currentAcctInstance *awsv1alpha1.Account, awsSetupClient awsclient.ClientV2, optInRegions string) (reconcile.Result, error) {
	// TODO: Implement opt-in region enablement for AWS SDK v2
	reqLogger.Info("Handling opt-in region enablement")
	return reconcile.Result{}, nil
}

func (r *AccountReconcilerV2) getAccountClaim(ctx context.Context, account *awsv1alpha1.Account) (*awsv1alpha1.AccountClaim, error) {
	accountClaim := &awsv1alpha1.AccountClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: account.Spec.ClaimLink, Namespace: account.Spec.ClaimLinkNamespace}, accountClaim)
	return accountClaim, err
}

func (r *AccountReconcilerV2) getSTSClientV2(ctx context.Context, reqLogger logr.Logger, accountClaim *awsv1alpha1.AccountClaim, awsSetupClient awsclient.ClientV2) (awsclient.ClientV2, *sts.AssumeRoleOutput, error) {
	// TODO: Implement STS client creation for AWS SDK v2
	reqLogger.Info("Getting STS client")
	return awsSetupClient, &sts.AssumeRoleOutput{}, nil
}

func (r *AccountReconcilerV2) handleIAMUserCreationV2(ctx context.Context, reqLogger logr.Logger, currentAcctInstance *awsv1alpha1.Account, awsSetupClient awsclient.ClientV2, namespace string) (reconcile.Result, *sts.AssumeRoleOutput, error) {
	// TODO: Implement IAM user creation for AWS SDK v2
	reqLogger.Info("Handling IAM user creation")
	return reconcile.Result{}, &sts.AssumeRoleOutput{}, nil
}

func (r *AccountReconcilerV2) initializeRegionsV2(ctx context.Context, reqLogger logr.Logger, currentAcctInstance *awsv1alpha1.Account, creds *sts.AssumeRoleOutput, amiOwner string) error {
	// TODO: Implement region initialization for AWS SDK v2
	reqLogger.Info("Initializing regions")
	return nil
}

func (r *AccountReconcilerV2) finalizeAccountV2(ctx context.Context, reqLogger logr.Logger, awsClient awsclient.ClientV2, account *awsv1alpha1.Account) {
	// TODO: Implement account finalization for AWS SDK v2
	reqLogger.Info("Finalizing account")
}

func (r *AccountReconcilerV2) statusUpdate(ctx context.Context, account *awsv1alpha1.Account) error {
	return r.Status().Update(ctx, account)
}

func ClaimAccountV2(r *AccountReconcilerV2, currentAcctInstance *awsv1alpha1.Account) error {
	// TODO: Implement account claiming for AWS SDK v2
	return nil
}

func isAwsOptInErrorV2(err error) bool {
	// TODO: Update error checking for AWS SDK v2 error types
	return false
}

func newBYOCAccountV2(account *awsv1alpha1.Account) bool {
	return account.Spec.BYOC
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccountReconcilerV2) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&awsv1alpha1.Account{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		Complete(r)
}
