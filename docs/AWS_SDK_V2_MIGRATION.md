# AWS SDK v2 Migration Guide

This document outlines the migration from AWS SDK v1 to AWS SDK v2 for the AWS Account Operator.

## Overview

The AWS Account Operator has been migrated to use AWS SDK v2, which provides:
- Better performance and reduced memory usage
- Improved error handling
- Context support for all operations
- Modular design with separate packages for each service

## Key Changes

### 1. Client Interface Changes

The main client interface has been updated to use AWS SDK v2:

**Before (AWS SDK v1):**
```go
import (
    "github.com/aws/aws-sdk-go/service/ec2"
    "github.com/aws/aws-sdk-go/service/iam"
    "github.com/aws/aws-sdk-go/service/sts"
)

// Client interface
type Client interface {
    RunInstances(*ec2.RunInstancesInput) (*ec2.Reservation, error)
    CreateUser(*iam.CreateUserInput) (*iam.CreateUserOutput, error)
    AssumeRole(*sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error)
}
```

**After (AWS SDK v2):**
```go
import (
    "context"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    "github.com/aws/aws-sdk-go-v2/service/iam"
    "github.com/aws/aws-sdk-go-v2/service/sts"
)

// ClientV2 interface
type ClientV2 interface {
    RunInstances(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
    CreateUser(ctx context.Context, params *iam.CreateUserInput, optFns ...func(*iam.Options)) (*iam.CreateUserOutput, error)
    AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error)
}
```

### 2. Context Support

All AWS SDK v2 operations now require a context parameter:

**Before:**
```go
result, err := client.RunInstances(&ec2.RunInstancesInput{
    ImageId: aws.String("ami-12345678"),
    MinCount: aws.Int64(1),
    MaxCount: aws.Int64(1),
})
```

**After:**
```go
result, err := client.RunInstances(ctx, &ec2.RunInstancesInput{
    ImageId: aws.String("ami-12345678"),
    MinCount: aws.Int32(1),
    MaxCount: aws.Int32(1),
})
```

### 3. Type Changes

Some types have changed between v1 and v2:

- `*int64` → `*int32` for counts and durations
- `*string` remains the same
- Error types are now more specific

### 4. Client Creation

**Before:**
```go
import (
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/session"
)

config := &aws.Config{
    Region:      aws.String("us-west-2"),
    Credentials: credentials.NewStaticCredentials(accessKey, secretKey, token),
}
sess, err := session.NewSession(config)
client := ec2.New(sess)
```

**After:**
```go
import (
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
)

cfg, err := config.LoadDefaultConfig(context.TODO(),
    config.WithRegion("us-west-2"),
    config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, token)),
)
client := ec2.NewFromConfig(cfg)
```

## Migration Steps

### 1. Update Dependencies

The `go.mod` file has been updated to include AWS SDK v2 dependencies:

```go
require (
    github.com/aws/aws-sdk-go-v2 v1.25.0
    github.com/aws/aws-sdk-go-v2/config v1.27.0
    github.com/aws/aws-sdk-go-v2/credentials v1.17.0
    github.com/aws/aws-sdk-go-v2/service/account v1.16.0
    github.com/aws/aws-sdk-go-v2/service/ec2 v1.150.0
    github.com/aws/aws-sdk-go-v2/service/iam v1.30.0
    github.com/aws/aws-sdk-go-v2/service/organizations v1.25.0
    github.com/aws/aws-sdk-go-v2/service/route53 v1.40.0
    github.com/aws/aws-sdk-go-v2/service/s3 v1.50.0
    github.com/aws/aws-sdk-go-v2/service/servicequotas v1.20.0
    github.com/aws/aws-sdk-go-v2/service/sts v1.25.0
    github.com/aws/aws-sdk-go-v2/service/support v1.19.0
)
```

### 2. Update Client Usage

Replace `awsclient.Client` with `awsclient.ClientV2` in your code:

**Before:**
```go
func (r *Reconciler) reconcileAccount(ctx context.Context, client awsclient.Client) error {
    result, err := client.RunInstances(&ec2.RunInstancesInput{
        ImageId: aws.String("ami-12345678"),
    })
    return err
}
```

**After:**
```go
func (r *Reconciler) reconcileAccount(ctx context.Context, client awsclient.ClientV2) error {
    result, err := client.RunInstances(ctx, &ec2.RunInstancesInput{
        ImageId: aws.String("ami-12345678"),
    })
    return err
}
```

### 3. Update STS Operations

**Before:**
```go
import stsclient "github.com/openshift/aws-account-operator/pkg/awsclient/sts"

client, creds, err := stsclient.AssumeRoleAndCreateClient(
    reqLogger, awsClientBuilder, currentAcctInstance, 
    client, awsSetupClient, region, roleToAssume, ccsRoleID)
```

**After:**
```go
import stsclient "github.com/openshift/aws-account-operator/pkg/awsclient/sts"

client, creds, err := stsclient.AssumeRoleAndCreateClientV2(
    reqLogger, awsClientBuilder, currentAcctInstance, 
    client, awsSetupClient, region, roleToAssume, ccsRoleID)
```

### 4. Update Error Handling

AWS SDK v2 provides more specific error types:

**Before:**
```go
import "github.com/aws/aws-sdk-go/aws/awserr"

if aerr, ok := err.(awserr.Error); ok {
    switch aerr.Code() {
    case "AccessDenied":
        // handle access denied
    }
}
```

**After:**
```go
import "github.com/aws/aws-sdk-go-v2/service/iam/types"

var accessDeniedErr *types.AccessDeniedException
if errors.As(err, &accessDeniedErr) {
    // handle access denied
}
```

## Backward Compatibility

The old AWS SDK v1 client (`awsclient.Client`) is still available for backward compatibility during the migration period. You can gradually migrate your code to use the new `awsclient.ClientV2` interface.

## Testing

When migrating, ensure you:

1. Update all function signatures to include context
2. Update all AWS SDK calls to use the new v2 interface
3. Update error handling to use the new error types
4. Test thoroughly with your AWS environment

## Known Issues

1. **S3 Batch Operations**: The `BatchDeleteBucketObjects` method is not yet fully implemented in the v2 client and returns an error.
2. **Metrics Collection**: The metrics collection for AWS API calls needs to be reimplemented for v2.

## Future Work

1. Complete the S3 batch operations implementation
2. Implement metrics collection for AWS SDK v2
3. Remove the old AWS SDK v1 client once migration is complete
4. Update all tests to use the new v2 client 