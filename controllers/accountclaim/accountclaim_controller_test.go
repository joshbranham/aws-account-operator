package accountclaim

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	apis "github.com/openshift/aws-account-operator/api"
	awsv1alpha1 "github.com/openshift/aws-account-operator/api/v1alpha1"
	"github.com/openshift/aws-account-operator/config"
	"github.com/openshift/aws-account-operator/pkg/awsclient/mock"
	"github.com/openshift/aws-account-operator/pkg/localmetrics"
	"github.com/openshift/aws-account-operator/test/fixtures"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AccountClaim", func() {
	var (
		name         = "testAccountClaim"
		namespace    = "myAccountClaimNamespace"
		accountClaim *awsv1alpha1.AccountClaim
		r            *AccountClaimReconciler
		ctrl         *gomock.Controller
		req          reconcile.Request
		configMap    *v1.ConfigMap
	)

	err := apis.AddToScheme(scheme.Scheme)
	if err != nil {
		fmt.Printf("failed adding apis to scheme in account controller tests")
	}
	localmetrics.Collector = localmetrics.NewMetricsCollector(nil)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		region := awsv1alpha1.AwsRegions{
			Name: "us-east-1",
		}
		accountClaim = &awsv1alpha1.AccountClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name,
				Namespace:         namespace,
				CreationTimestamp: metav1.Time{},
			},
			Spec: awsv1alpha1.AccountClaimSpec{
				LegalEntity: awsv1alpha1.LegalEntity{
					Name: "LegalCorp. Inc.",
					ID:   "abcdefg123456",
				},
				AccountLink: "osd-creds-mgmt-aaabbb",
				Aws: awsv1alpha1.Aws{
					Regions: []awsv1alpha1.AwsRegions{region},
				},
			},
		}
		req = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		// Create the reconciler with a mocking AWS client IBuilder.
		r = &AccountClaimReconciler{
			// Test cases need to set fakeClient.
			Scheme: scheme.Scheme,
			awsClientBuilder: &mock.Builder{
				MockController: ctrl,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	When("Reconciling an AccountClaim", func() {
		It("should reconcile correctly", func() {
			// Objects to track in the fake client.
			objs := []runtime.Object{accountClaim}
			r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

			_, err := r.Reconcile(context.TODO(), req)

			Expect(err).NotTo(HaveOccurred())
			ac := awsv1alpha1.AccountClaim{}
			err = r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &ac)
			Expect(err).NotTo(HaveOccurred())
			Expect(ac.Spec).To(Equal(accountClaim.Spec))
		})

		Context("AccountClaim is marked for Deletion", func() {

			var (
				objs              []runtime.Object
				orgAccessRoleName string
				roleSessionName   string
				orgAccessArn      string
			)

			BeforeEach(func() {
				orgAccessRoleName = "OrganizationAccountAccessRole"
				roleSessionName = "awsAccountOperator"
				orgAccessArn = "arn:aws:iam:::role/OrganizationAccountAccessRole"
				accountClaim.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				accountClaim.SetFinalizers(append(accountClaim.GetFinalizers(), accountClaimFinalizer))

				account := &awsv1alpha1.Account{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "osd-creds-mgmt-aaabbb",
						Namespace: "aws-account-operator",
					},
					Spec: awsv1alpha1.AccountSpec{
						LegalEntity: awsv1alpha1.LegalEntity{
							Name: "LegalCorp. Inc.",
							ID:   "abcdefg123456",
						},
					},
				}

				objs = []runtime.Object{accountClaim, account}
			})

			It("should delete AccountClaim", func() {
				r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

				mockAWSClient := mock.GetMockClient(r.awsClientBuilder)
				// Create empty empy aws responses.
				lhzo := &route53.ListHostedZonesOutput{
					HostedZones: []*route53.HostedZone{},
					IsTruncated: aws.Bool(false),
				}
				lbo := &s3.ListBucketsOutput{
					Buckets: []*s3.Bucket{},
				}
				dvpcesco := &ec2.DescribeVpcEndpointServiceConfigurationsOutput{
					ServiceConfigurations: []*ec2.ServiceConfiguration{},
				}
				dso := &ec2.DescribeSnapshotsOutput{
					Snapshots: []*ec2.Snapshot{},
				}
				dvo := &ec2.DescribeVolumesOutput{
					Volumes: []*ec2.Volume{},
				}

				mockAWSClient.EXPECT().AssumeRole(&sts.AssumeRoleInput{
					DurationSeconds: aws.Int64(3600),
					RoleArn:         &orgAccessArn,
					RoleSessionName: &roleSessionName,
				}).Return(&sts.AssumeRoleOutput{
					AssumedRoleUser: &sts.AssumedRoleUser{
						Arn:           aws.String(fmt.Sprintf("aws:::%s/%s", orgAccessRoleName, roleSessionName)),
						AssumedRoleId: aws.String(fmt.Sprintf("%s/%s", orgAccessRoleName, roleSessionName)),
					},
					Credentials: &sts.Credentials{
						AccessKeyId:     aws.String("ACCESS_KEY"),
						SecretAccessKey: aws.String("SECRET_KEY"),
						SessionToken:    aws.String("SESSION_TOKEN"),
					},
					PackedPolicySize: aws.Int64(40),
				}, nil)
				mockAWSClient.EXPECT().ListHostedZones(gomock.Any()).Return(lhzo, nil)
				mockAWSClient.EXPECT().ListBuckets(gomock.Any()).Return(lbo, nil)
				mockAWSClient.EXPECT().DescribeVpcEndpointServiceConfigurations(gomock.Any()).Return(dvpcesco, nil)
				mockAWSClient.EXPECT().DescribeSnapshots(gomock.Any()).Return(dso, nil)
				mockAWSClient.EXPECT().DescribeVolumes(gomock.Any()).Return(dvo, nil)

				// Confirm that the accountclaim exists from the client's perspective
				ac := awsv1alpha1.AccountClaim{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &ac)
				Expect(err).NotTo(HaveOccurred())

				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).ToNot(HaveOccurred())

				// With the finalizer removed, the AccountClaim should have been deleted
				err = r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &ac)
				Expect(k8serr.IsNotFound(err)).To(BeTrue())

				// Ensure the non-ccs account has been reset as expected.
				acc := awsv1alpha1.Account{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: ac.Spec.AccountLink, Namespace: awsv1alpha1.AccountCrNamespace}, &acc)
				Expect(err).NotTo(HaveOccurred())
				Expect(acc.Spec.ClaimLink).To(BeEmpty())
				Expect(acc.Spec.ClaimLinkNamespace).To(BeEmpty())
				Expect(acc.Status.State).To(Equal(string(awsv1alpha1.AccountReady)))
				Expect(acc.Status.Reused).To(BeTrue())
			})

			It("should retry on a conflict error", func() {
				r.Client = &possiblyErroringFakeCtrlRuntimeClient{
					fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build(),
					true,
				}

				mockAWSClient := mock.GetMockClient(r.awsClientBuilder)
				// Create empty empy aws responses.
				lhzo := &route53.ListHostedZonesOutput{
					HostedZones: []*route53.HostedZone{},
					IsTruncated: aws.Bool(false),
				}
				lbo := &s3.ListBucketsOutput{
					Buckets: []*s3.Bucket{},
				}
				dvpcesco := &ec2.DescribeVpcEndpointServiceConfigurationsOutput{
					ServiceConfigurations: []*ec2.ServiceConfiguration{},
				}
				dso := &ec2.DescribeSnapshotsOutput{
					Snapshots: []*ec2.Snapshot{},
				}
				dvo := &ec2.DescribeVolumesOutput{
					Volumes: []*ec2.Volume{},
				}

				mockAWSClient.EXPECT().AssumeRole(&sts.AssumeRoleInput{
					DurationSeconds: aws.Int64(3600),
					RoleArn:         &orgAccessArn,
					RoleSessionName: &roleSessionName,
				}).Return(&sts.AssumeRoleOutput{
					AssumedRoleUser: &sts.AssumedRoleUser{
						Arn:           aws.String(fmt.Sprintf("aws:::%s/%s", orgAccessRoleName, roleSessionName)),
						AssumedRoleId: aws.String(fmt.Sprintf("%s/%s", orgAccessRoleName, roleSessionName)),
					},
					Credentials: &sts.Credentials{
						AccessKeyId:     aws.String("ACCESS_KEY"),
						SecretAccessKey: aws.String("SECRET_KEY"),
						SessionToken:    aws.String("SESSION_TOKEN"),
					},
					PackedPolicySize: aws.Int64(40),
				}, nil)
				mockAWSClient.EXPECT().ListHostedZones(gomock.Any()).Return(lhzo, nil)
				mockAWSClient.EXPECT().ListBuckets(gomock.Any()).Return(lbo, nil)
				mockAWSClient.EXPECT().DescribeVpcEndpointServiceConfigurations(gomock.Any()).Return(dvpcesco, nil)
				mockAWSClient.EXPECT().DescribeSnapshots(gomock.Any()).Return(dso, nil)
				mockAWSClient.EXPECT().DescribeVolumes(gomock.Any()).Return(dvo, nil)

				_, err := r.Reconcile(context.TODO(), req)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("account CR modified during reset: Conflict"))

				// Ensure we haven't removed the finalizer.
				ac := awsv1alpha1.AccountClaim{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &ac)
				Expect(err).NotTo(HaveOccurred())
				Expect(ac.Finalizers).To(Equal(accountClaim.GetFinalizers()))
			})

			It("should handle aws cleanup errors", func() {
				r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

				mockAWSClient := mock.GetMockClient(r.awsClientBuilder)
				// Use a bogus error, just so we can fail AWS calls.
				theErr := awserr.NewBatchError("foo", "bar", []error{})
				mockAWSClient.EXPECT().AssumeRole(&sts.AssumeRoleInput{
					DurationSeconds: aws.Int64(3600),
					RoleArn:         &orgAccessArn,
					RoleSessionName: &roleSessionName,
				}).Return(&sts.AssumeRoleOutput{
					AssumedRoleUser: &sts.AssumedRoleUser{
						Arn:           aws.String(fmt.Sprintf("aws:::%s/%s", orgAccessRoleName, roleSessionName)),
						AssumedRoleId: aws.String(fmt.Sprintf("%s/%s", orgAccessRoleName, roleSessionName)),
					},
					Credentials: &sts.Credentials{
						AccessKeyId:     aws.String("ACCESS_KEY"),
						SecretAccessKey: aws.String("SECRET_KEY"),
						SessionToken:    aws.String("SESSION_TOKEN"),
					},
					PackedPolicySize: aws.Int64(40),
				}, nil)
				mockAWSClient.EXPECT().ListHostedZones(gomock.Any()).Return(nil, theErr)
				mockAWSClient.EXPECT().ListBuckets(gomock.Any()).Return(nil, theErr)
				mockAWSClient.EXPECT().DescribeVpcEndpointServiceConfigurations(gomock.Any()).Return(nil, theErr)
				mockAWSClient.EXPECT().DescribeSnapshots(gomock.Any()).Return(nil, theErr)
				mockAWSClient.EXPECT().DescribeVolumes(gomock.Any()).Return(nil, theErr)

				_, err := r.Reconcile(context.TODO(), req)

				Expect(err).To(HaveOccurred())

				// Ensure we haven't removed the finalizer.
				ac := awsv1alpha1.AccountClaim{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &ac)
				Expect(err).NotTo(HaveOccurred())
				Expect(ac.Finalizers).To(Equal(accountClaim.GetFinalizers()))
			})

			It("should do nothing when there are additional finalizers present", func() {
				accountClaim.SetFinalizers(append(accountClaim.GetFinalizers(), "another.blocking.finalizer"))
				r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()
				_, err := r.Reconcile(context.TODO(), req)

				Expect(err).NotTo(HaveOccurred())

				// validate that all finalizers are still there
				ac := awsv1alpha1.AccountClaim{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &ac)
				Expect(err).NotTo(HaveOccurred())
				Expect(ac.Finalizers).To(Equal(accountClaim.GetFinalizers()))

			})
		})
		When("accountClaim.Spec.FleetManagerConfig.TrustedARN & accountClaim.Spec.AccountPool defined", func() {
			BeforeEach(func() {
				// Set up the test data to meet the condition
				accountClaim.Spec.FleetManagerConfig.TrustedARN = "arn:aws:iam::123456789012:role/testRoleName"
				accountClaim.Spec.AccountPool = "testAccountPool"
				accountClaim.Spec.AccountOU = "ou-0wd6-kcuacjuw"

			})
			It("should reconcile correctly when TrustedARN and AccountPool conditions are met", func() {
				mockAWSClient := mock.GetMockClient(r.awsClientBuilder)
				req = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      accountClaim.Name,
						Namespace: namespace,
					},
				}
				dummySecretRef := awsv1alpha1.SecretRef{
					Name:      awsSTSSecret,
					Namespace: namespace,
				}
				accountClaim.Spec.AwsCredentialSecret = dummySecretRef
				configMap = &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      awsv1alpha1.DefaultConfigMap,
						Namespace: awsv1alpha1.AccountCrNamespace,
					},
					Data: map[string]string{
						"feature.accountclaim_fleet_manager_trusted_arn": "true",
					},
				}
				accounts := []*awsv1alpha1.Account{}
				accounts = append(accounts, &awsv1alpha1.Account{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "osd-creds-mgmt-aaabbb",
						Namespace:         "aws-account-operator",
						CreationTimestamp: metav1.Time{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "AccountPool",
							},
						},
					},
					Spec: awsv1alpha1.AccountSpec{
						AccountPool:        "testAccountPool",
						IAMUserSecret:      "test-secret",
						AwsAccountID:       "123456789012",
						ClaimLink:          accountClaim.Name,
						ClaimLinkNamespace: accountClaim.Namespace,
						LegalEntity: awsv1alpha1.LegalEntity{
							Name: accountClaim.Name,
							ID:   "abcdefg",
						},
					},
					Status: awsv1alpha1.AccountStatus{
						State:   AccountReady,
						Claimed: false,
					},
				})
				objs := []runtime.Object{accountClaim, accounts[0], configMap}

				r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()
				roleName := "testRoleName"
				orgAccessRoleName := "OrganizationAccountAccessRole"
				orgAccessArn := config.GetIAMArn(accounts[0].Spec.AwsAccountID, config.AwsResourceTypeRole, orgAccessRoleName)
				roleSessionName := "awsAccountOperator"

				mockAWSClient.EXPECT().AssumeRole(&sts.AssumeRoleInput{
					DurationSeconds: aws.Int64(3600),
					RoleArn:         &orgAccessArn,
					RoleSessionName: &roleSessionName,
				}).Return(&sts.AssumeRoleOutput{
					AssumedRoleUser: &sts.AssumedRoleUser{
						Arn:           aws.String(fmt.Sprintf("aws:::%s/%s", orgAccessRoleName, roleSessionName)),
						AssumedRoleId: aws.String(fmt.Sprintf("%s/%s", orgAccessRoleName, roleSessionName)),
					},
					Credentials: &sts.Credentials{
						AccessKeyId:     aws.String("ACCESS_KEY"),
						SecretAccessKey: aws.String("SECRET_KEY"),
						SessionToken:    aws.String("SESSION_TOKEN"),
					},
					PackedPolicySize: aws.Int64(40),
				}, nil)

				mockAWSClient.EXPECT().GetRole(gomock.Any()).Return(&iam.GetRoleOutput{}, nil)
				mockAWSClient.EXPECT().ListRolePolicies(gomock.Any()).Return(&iam.ListRolePoliciesOutput{}, nil)
				mockAWSClient.EXPECT().DeleteRole(gomock.Any()).Return(&iam.DeleteRoleOutput{}, nil)
				mockAWSClient.EXPECT().ListUsersPages(gomock.Any(), gomock.Any()).Return(nil)

				expectedCreateRoleOutput := &iam.CreateRoleOutput{
					Role: &iam.Role{
						RoleName:    aws.String(roleName),
						Arn:         aws.String("arn:aws:iam::123456789012:role/" + roleName),
						Description: aws.String("Created by AAO"),
					},
				}

				mockAWSClient.EXPECT().CreateRole(gomock.Any()).Return(expectedCreateRoleOutput, nil)
				mockAWSClient.EXPECT().PutRolePolicy(gomock.Any()).Return(nil, nil)

				for i := 0; i < 3; i++ {
					_, err = r.Reconcile(context.TODO(), req)
				}

				ac := awsv1alpha1.AccountClaim{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &ac)
				Expect(ac.Status.State).To(Equal(awsv1alpha1.ClaimStatusReady))

				account := awsv1alpha1.Account{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: accounts[0].Name, Namespace: accounts[0].Namespace}, &account)
				Expect(account.Spec.IAMUserSecret).To(Equal(""))

				IAMUsersecret := v1.Secret{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: account.Spec.IAMUserSecret, Namespace: awsv1alpha1.AccountCrNamespace}, &IAMUsersecret)
				Expect(err).To(HaveOccurred())

				roleSecret := v1.Secret{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: accountClaim.Spec.AwsCredentialSecret.Name, Namespace: accountClaim.Spec.AwsCredentialSecret.Namespace}, &roleSecret)
				Expect(err).ToNot(HaveOccurred())

			})

		})

		When("Accountclaim is BYOC", func() {

			BeforeEach(func() {
				accountClaim.SetFinalizers(append(accountClaim.GetFinalizers(), accountClaimFinalizer))
				accountClaim.Spec.BYOC = true
				accountClaim.Spec.AccountLink = ""
			})

			It("should fail validation", func() {
				// fail validation if BYOC is not associated with an account
				accountClaim.Spec.BYOCAWSAccountID = ""

				r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(accountClaim).Build()

				_, err := r.Reconcile(context.TODO(), req)

				Expect(err).To(HaveOccurred())
				ac := awsv1alpha1.AccountClaim{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &ac)
				Expect(err).NotTo(HaveOccurred())
				Expect(ac.Status.State).To(Equal(awsv1alpha1.ClaimStatusError))
			})

			It("Should create a BYOC Account", func() {
				dummySecretRef := awsv1alpha1.SecretRef{
					Name:      "name",
					Namespace: "namespace",
				}
				accountClaim.Spec.BYOCSecretRef = dummySecretRef
				accountClaim.Spec.AwsCredentialSecret = dummySecretRef
				accountClaim.Spec.BYOCAWSAccountID = "123456"

				r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(accountClaim).Build()

				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).NotTo(HaveOccurred())

				ac := awsv1alpha1.AccountClaim{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &ac)
				Expect(err).NotTo(HaveOccurred())

				account := awsv1alpha1.Account{}
				err = r.Get(context.TODO(), types.NamespacedName{Name: ac.Spec.AccountLink, Namespace: awsv1alpha1.AccountCrNamespace}, &account)

				Expect(err).NotTo(HaveOccurred())
				Expect(account.Spec.BYOC).To(BeTrue())
				Expect(account.Spec.LegalEntity.ID).To(Equal(accountClaim.Spec.LegalEntity.ID))
				Expect(account.Spec.AwsAccountID).To(Equal(accountClaim.Spec.BYOCAWSAccountID))
			})
		})
	})
})

type possiblyErroringFakeCtrlRuntimeClient struct {
	client.Client
	shouldError bool
}

func (p *possiblyErroringFakeCtrlRuntimeClient) Update(
	ctx context.Context,
	acc client.Object,
	opts ...client.UpdateOption) error {
	if p.shouldError {
		return fixtures.Conflict
	}
	return p.Client.Update(ctx, acc)
}

var _ = Describe("Mutiple AccountPools Claim", func() {
	var (
		namespace = awsv1alpha1.AccountCrNamespace // Can't set custom namespace as getUnclaimedAccount has this hardcoded

		defaultAccountName     = "default-account"
		defaultClaimName       = "default-accountclaim"
		defaultAccountPoolName = "my-default-accountpool"

		sqAccountName     = "service-quota-account"
		sqClaimName       = "service-quota-accountclaim"
		sqAccountPoolName = "my-service-quota-accountpool"

		accounts      []*awsv1alpha1.Account
		accountClaims []*awsv1alpha1.AccountClaim
		configMap     *v1.ConfigMap

		r              *AccountClaimReconciler
		ctrl           *gomock.Controller
		req            reconcile.Request
		reconcileCount = 2
	)

	err := apis.AddToScheme(scheme.Scheme)
	if err != nil {
		fmt.Printf("failed adding apis to scheme in account controller tests")
	}
	localmetrics.Collector = localmetrics.NewMetricsCollector(nil)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		myYaml := `
        my-default-accountpool:
          default: true`
		configMap = &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:        awsv1alpha1.DefaultConfigMap,
				Namespace:   awsv1alpha1.AccountCrNamespace,
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			Data: map[string]string{
				"accountpool": myYaml,
			},
		}
		// Create the reconciler with a mocking AWS client IBuilder.
		r = &AccountClaimReconciler{
			// Test cases need to set fakeClient.
			Scheme: scheme.Scheme,
			awsClientBuilder: &mock.Builder{
				MockController: ctrl,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	When("Reconciling an AccountClaim", func() {
		When("Only Default AccountPool Account Exists", func() {

			BeforeEach(func() {
				accounts = []*awsv1alpha1.Account{}
				accountClaims = []*awsv1alpha1.AccountClaim{}

				accounts = append(accounts, &awsv1alpha1.Account{
					ObjectMeta: metav1.ObjectMeta{
						Name:              defaultAccountName,
						Namespace:         namespace,
						CreationTimestamp: metav1.Time{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "AccountPool",
							},
						},
					},
					Spec: awsv1alpha1.AccountSpec{
						AccountPool: "",
					},
					Status: awsv1alpha1.AccountStatus{
						State:   AccountReady,
						Claimed: false,
					},
				})
			})

			AfterEach(func() {
				accounts = []*awsv1alpha1.Account{}
				accountClaims = []*awsv1alpha1.AccountClaim{}
			})

			When("The first account is claimed under a different legal entity", func() {
				BeforeEach(func() {
					legalEntity1 := awsv1alpha1.LegalEntity{
						Name: "test1",
						ID:   "abcdefg",
					}
					legalEntity2 := awsv1alpha1.LegalEntity{
						Name: "test2",
						ID:   "hijklmno",
					}

					accounts[0].Spec.LegalEntity = legalEntity1
					accounts[0].Status.Reused = true
					accounts = append(accounts, &awsv1alpha1.Account{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "account-two",
							Namespace:         namespace,
							CreationTimestamp: metav1.Time{},
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "AccountPool",
								},
							},
						},
						Spec: awsv1alpha1.AccountSpec{
							AccountPool: "",
							LegalEntity: legalEntity2,
						},
						Status: awsv1alpha1.AccountStatus{
							State:   AccountReady,
							Claimed: false,
							Reused:  true,
						},
					})

					accountClaims = append(accountClaims, &awsv1alpha1.AccountClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:              sqClaimName,
							Namespace:         namespace,
							CreationTimestamp: metav1.Time{},
							Finalizers:        []string{accountClaimFinalizer},
						},
						Spec: awsv1alpha1.AccountClaimSpec{
							LegalEntity: legalEntity2,
						},
					})

					objs := []runtime.Object{configMap, accountClaims[0], accounts[0], accounts[1]}
					r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()
				})

				It("should claim the legalEntity2 account", func() {
					req = reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      sqClaimName,
							Namespace: namespace,
						},
					}

					for i := 0; i < reconcileCount; i++ {
						_, err := r.Reconcile(context.TODO(), req)
						Expect(err).NotTo(HaveOccurred())
					}

					acc := awsv1alpha1.Account{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: defaultAccountName, Namespace: namespace}, &acc)
					Expect(err).NotTo(HaveOccurred())
					Expect(acc.Spec.ClaimLink).To(BeEmpty())
					Expect(acc.Spec.ClaimLinkNamespace).To(BeEmpty())
					Expect(acc.Status.State).To(Equal(string(awsv1alpha1.AccountReady)))

					err = r.Get(context.TODO(), types.NamespacedName{Name: "account-two", Namespace: namespace}, &acc)
					Expect(err).NotTo(HaveOccurred())
					Expect(acc.Spec.ClaimLink).To(Equal(sqClaimName))
					Expect(acc.Spec.ClaimLinkNamespace).To(Equal(namespace))
					Expect(acc.Status.State).To(Equal(string(awsv1alpha1.AccountReady)))

					claim := awsv1alpha1.AccountClaim{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: sqClaimName, Namespace: namespace}, &claim)
					Expect(err).NotTo(HaveOccurred())
					Expect(claim.Spec.AccountLink).To(Equal(acc.Name))
				})
			})

			When("Multiple accounts are available", func() {
				BeforeEach(func() {
					legalEntity := awsv1alpha1.LegalEntity{
						Name: "test1",
						ID:   "abcdefg",
					}

					accounts[0].Spec.LegalEntity = legalEntity
					accounts[0].Status.Reused = true
					accounts = append(accounts, &awsv1alpha1.Account{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "a-not-me",
							Namespace:         namespace,
							CreationTimestamp: metav1.Time{},
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "AccountPool",
								},
							},
						},
						Spec: awsv1alpha1.AccountSpec{
							AccountPool: "",
							LegalEntity: legalEntity,
						},
						Status: awsv1alpha1.AccountStatus{
							State:   AccountReady,
							Claimed: false,
							Reused:  false,
						},
					})

					accountClaims = append(accountClaims, &awsv1alpha1.AccountClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:              defaultClaimName,
							Namespace:         namespace,
							CreationTimestamp: metav1.Time{},
							Finalizers:        []string{accountClaimFinalizer},
						},
						Spec: awsv1alpha1.AccountClaimSpec{
							LegalEntity: legalEntity,
						},
					})

					objs := []runtime.Object{configMap, accountClaims[0], accounts[0], accounts[1]}
					// for _, a := range accounts {
					//   objs = append(objs, a)
					// }

					r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()
				})
				It("Should claim the reused account instead of the unused", func() {
					req = reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      defaultClaimName,
							Namespace: namespace,
						},
					}

					for i := 0; i < reconcileCount; i++ {
						_, err := r.Reconcile(context.TODO(), req)
						Expect(err).NotTo(HaveOccurred())
					}

					acc := awsv1alpha1.Account{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: defaultAccountName, Namespace: namespace}, &acc)
					Expect(err).NotTo(HaveOccurred())
					Expect(acc.Spec.ClaimLink).To(Equal(defaultClaimName))
					Expect(acc.Spec.ClaimLinkNamespace).To(Equal(namespace))
					Expect(acc.Status.State).To(Equal(string(awsv1alpha1.AccountReady)))
				})
			})

			When("We create a non-default claim", func() {
				It("should NOT claim the default account", func() {
					accountClaims = append(accountClaims, &awsv1alpha1.AccountClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:              sqClaimName,
							Namespace:         namespace,
							CreationTimestamp: metav1.Time{},
							Finalizers:        []string{accountClaimFinalizer},
						},
						Spec: awsv1alpha1.AccountClaimSpec{
							AccountPool: sqAccountPoolName,
						},
					})

					objs := []runtime.Object{configMap, accountClaims[0], accounts[0]}
					r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

					req = reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      sqClaimName,
							Namespace: namespace,
						},
					}

					for i := 0; i < reconcileCount; i++ {
						_, err := r.Reconcile(context.TODO(), req)
						if i == 0 {
							Expect(err).NotTo(HaveOccurred())
						} else {
							Expect(err).To(HaveOccurred())
						}
					}

					acc := awsv1alpha1.Account{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: defaultAccountName, Namespace: namespace}, &acc)
					Expect(err).NotTo(HaveOccurred())
					Expect(acc.Spec.ClaimLink).To(BeEmpty())
					Expect(acc.Spec.ClaimLinkNamespace).To(BeEmpty())
					Expect(acc.Status.State).To(Equal(string(awsv1alpha1.AccountReady)))
				})
			})
		})

		When("both account types are available", func() {
			// Needs default + non-default account
			BeforeEach(func() {
				accounts = []*awsv1alpha1.Account{}
				accountClaims = []*awsv1alpha1.AccountClaim{}

				accounts = append(accounts, &awsv1alpha1.Account{
					ObjectMeta: metav1.ObjectMeta{
						Name:              defaultAccountName,
						Namespace:         namespace,
						CreationTimestamp: metav1.Time{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "AccountPool",
							},
						},
					},
					Spec: awsv1alpha1.AccountSpec{
						AccountPool: defaultAccountPoolName,
					},
					Status: awsv1alpha1.AccountStatus{
						State:   AccountReady,
						Claimed: false,
					},
				})
				accounts = append(accounts, &awsv1alpha1.Account{
					ObjectMeta: metav1.ObjectMeta{
						Name:              sqAccountName,
						Namespace:         namespace,
						CreationTimestamp: metav1.Time{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "AccountPool",
							},
						},
					},
					Spec: awsv1alpha1.AccountSpec{
						AccountPool: sqAccountPoolName,
					},
					Status: awsv1alpha1.AccountStatus{
						State:   AccountReady,
						Claimed: false,
					},
				})
			})

			AfterEach(func() {
				accounts = []*awsv1alpha1.Account{}
				accountClaims = []*awsv1alpha1.AccountClaim{}
			})

			When("we create a non-default claim", func() {
				It("should claim the non-default account", func() {
					// Needs non-default accountclaim
					accountClaims = append(accountClaims, &awsv1alpha1.AccountClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:              sqClaimName,
							Namespace:         namespace,
							CreationTimestamp: metav1.Time{},
							Finalizers:        []string{accountClaimFinalizer},
						},
						Spec: awsv1alpha1.AccountClaimSpec{
							AccountPool: sqAccountPoolName,
						},
					})

					objs := []runtime.Object{configMap, accountClaims[0], accounts[0], accounts[1]}
					r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

					req = reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      sqClaimName,
							Namespace: namespace,
						},
					}

					for i := 0; i < reconcileCount; i++ {
						_, err := r.Reconcile(context.TODO(), req)
						Expect(err).NotTo(HaveOccurred())
					}

					// Default
					acc := awsv1alpha1.Account{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: defaultAccountName, Namespace: namespace}, &acc)
					Expect(err).NotTo(HaveOccurred())
					Expect(acc.Spec.ClaimLink).To(BeEmpty())
					Expect(acc.Spec.ClaimLinkNamespace).To(BeEmpty())
					Expect(acc.Status.State).To(Equal(string(awsv1alpha1.AccountReady)))

					// SQ
					acc = awsv1alpha1.Account{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: sqAccountName, Namespace: namespace}, &acc)
					Expect(err).NotTo(HaveOccurred())
					Expect(acc.Spec.ClaimLink).To(Equal(sqClaimName))
					Expect(acc.Spec.ClaimLinkNamespace).To(Equal(namespace))
					Expect(acc.Status.State).To(Equal(string(awsv1alpha1.AccountReady)))

					claim := awsv1alpha1.AccountClaim{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: sqClaimName, Namespace: namespace}, &claim)
					Expect(err).NotTo(HaveOccurred())
					Expect(claim.Spec.AccountLink).To(Equal(sqAccountName))
				})
			})
			When("we create an explicit claim to the default account pool name", func() {
				It("should claim the default account", func() {
					accountClaims = append(accountClaims, &awsv1alpha1.AccountClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:              defaultClaimName,
							Namespace:         namespace,
							CreationTimestamp: metav1.Time{},
							Finalizers:        []string{accountClaimFinalizer},
						},
						Spec: awsv1alpha1.AccountClaimSpec{
							AccountPool: defaultAccountPoolName,
						},
					})

					objs := []runtime.Object{configMap, accountClaims[0], accounts[0], accounts[1]}
					r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

					req = reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      defaultClaimName,
							Namespace: namespace,
						},
					}

					for i := 0; i < reconcileCount; i++ {
						_, err := r.Reconcile(context.TODO(), req)
						Expect(err).NotTo(HaveOccurred())
					}

					// Default
					acc := awsv1alpha1.Account{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: defaultAccountName, Namespace: namespace}, &acc)
					Expect(err).NotTo(HaveOccurred())
					Expect(acc.Spec.ClaimLink).To(Equal(defaultClaimName))
					Expect(acc.Spec.ClaimLinkNamespace).To(Equal(namespace))
					Expect(acc.Status.State).To(Equal(string(awsv1alpha1.AccountReady)))

					// SQ
					acc = awsv1alpha1.Account{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: sqAccountName, Namespace: namespace}, &acc)
					Expect(err).NotTo(HaveOccurred())
					Expect(acc.Spec.ClaimLink).To(BeEmpty())
					Expect(acc.Spec.ClaimLinkNamespace).To(BeEmpty())
					Expect(acc.Status.State).To(Equal(string(awsv1alpha1.AccountReady)))

					claim := awsv1alpha1.AccountClaim{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: defaultClaimName, Namespace: namespace}, &claim)
					Expect(err).NotTo(HaveOccurred())
					Expect(claim.Spec.AccountLink).To(Equal(defaultAccountName))
				})
			})

			When("we create a blank claim (by not specifying an AccountPool in the claim spec)", func() {
				It("should claim the default account", func() {
					accountClaims = append(accountClaims, &awsv1alpha1.AccountClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:              defaultClaimName,
							Namespace:         namespace,
							CreationTimestamp: metav1.Time{},
							Finalizers:        []string{accountClaimFinalizer},
						},
						Spec: awsv1alpha1.AccountClaimSpec{},
					})

					objs := []runtime.Object{configMap, accountClaims[0], accounts[0], accounts[1]}
					r.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

					req = reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      defaultClaimName,
							Namespace: namespace,
						},
					}

					for i := 0; i < reconcileCount; i++ {
						_, err := r.Reconcile(context.TODO(), req)
						Expect(err).NotTo(HaveOccurred())
					}

					// Default
					acc := awsv1alpha1.Account{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: defaultAccountName, Namespace: namespace}, &acc)
					Expect(err).NotTo(HaveOccurred())
					Expect(acc.Spec.ClaimLink).To(Equal(defaultClaimName))
					Expect(acc.Spec.ClaimLinkNamespace).To(Equal(namespace))
					Expect(acc.Status.State).To(Equal(string(awsv1alpha1.AccountReady)))

					// SQ
					acc = awsv1alpha1.Account{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: sqAccountName, Namespace: namespace}, &acc)
					Expect(err).NotTo(HaveOccurred())
					Expect(acc.Spec.ClaimLink).To(BeEmpty())
					Expect(acc.Spec.ClaimLinkNamespace).To(BeEmpty())
					Expect(acc.Status.State).To(Equal(string(awsv1alpha1.AccountReady)))

					claim := awsv1alpha1.AccountClaim{}
					err = r.Get(context.TODO(), types.NamespacedName{Name: defaultClaimName, Namespace: namespace}, &claim)
					Expect(err).NotTo(HaveOccurred())
					Expect(claim.Spec.AccountLink).To(Equal(defaultAccountName))
				})
			})
		})
	})
})
