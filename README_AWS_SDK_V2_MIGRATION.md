# AWS SDK v2 Migration Status

This document tracks the progress of migrating the AWS Account Operator from AWS SDK for Go v1 to AWS SDK for Go v2.

## Migration Strategy

The migration is being done incrementally to maintain backward compatibility:

1. **Phase 1**: Create parallel AWS SDK v2 client interface and implementation
2. **Phase 2**: Migrate controllers to use AWS SDK v2 client interface
3. **Phase 3**: Update tests to use AWS SDK v2 clients
4. **Phase 4**: Remove AWS SDK v1 client code

## Current Status

### ✅ Completed

#### Phase 1: AWS SDK v2 Client Implementation
- [x] Created `pkg/awsclient/client_v2.go` with AWS SDK v2 client interface
- [x] Implemented all major AWS service clients (Organizations, STS, EC2, IAM, S3, Route53, ServiceQuotas, Support)
- [x] Added client builder pattern for AWS SDK v2
- [x] Implemented context-aware method signatures
- [x] Added proper error handling for AWS SDK v2 error types

#### Phase 2: Controller Migration (In Progress)
- [x] Created `controllers/account/account_controller_v2.go` with AWS SDK v2 client interface
- [x] Migrated main Reconcile method to use context.Context and AWS SDK v2 client
- [x] Implemented key methods:
  - [x] `handleAWSClientErrorV2` - AWS SDK v2 error handling
  - [x] `nonCCSAssignAccountIDV2` - Account ID assignment
  - [x] `BuildAccountV2` - Account creation logic
  - [x] `CreateAccountV2` - AWS Organizations account creation
  - [x] `TagAccountV2` - Account tagging
  - [x] `addFinalizer` and `removeFinalizer` - Finalizer management
- [x] Fixed AWS SDK v2 type usage (organizations.Tag, CreateAccountStatus fields)
- [x] Resolved import conflicts and constant redeclaration issues

### 🔄 In Progress

#### Phase 2: Controller Migration (Continued)
- [ ] Complete remaining stubbed methods in `account_controller_v2.go`:
  - [ ] `handleAccountInitializingRegionsV2` - Region initialization logic
  - [ ] `initializeNewCCSAccountV2` - CCS account initialization
  - [ ] `HandleNonCCSPendingVerificationV2` - Pending verification logic
  - [ ] `handleOptInRegionEnablementV2` - Opt-in region enablement
  - [ ] `getSTSClientV2` - STS client creation
  - [ ] `handleIAMUserCreationV2` - IAM user creation
  - [ ] `initializeRegionsV2` - Region initialization
  - [ ] `finalizeAccountV2` - Account finalization
  - [ ] `ClaimAccountV2` - Account claiming
  - [ ] `isAwsOptInErrorV2` - Error checking for AWS SDK v2

### 📋 Next Steps

#### Immediate Next Steps
1. **Complete Account Controller v2**: Implement all remaining stubbed methods
2. **Migrate AccountClaim Controller**: Create `accountclaim_controller_v2.go` using AWS SDK v2 client interface
3. **Update STS Client**: Ensure `pkg/awsclient/sts/sts_assume_role.go` has proper AWS SDK v2 implementation

#### Phase 3: Test Migration
- [ ] Update unit tests to use AWS SDK v2 clients
- [ ] Update integration tests to use AWS SDK v2 clients
- [ ] Update mock implementations for AWS SDK v2

#### Phase 4: Cleanup
- [ ] Remove AWS SDK v1 client code
- [ ] Update all imports to use AWS SDK v2
- [ ] Remove AWS SDK v1 dependencies from go.mod

## Key Changes Made

### AWS SDK v2 Client Interface
- All methods now accept `context.Context` as first parameter
- Error handling uses `smithy.OperationError` instead of `awserr.Error`
- Type names have changed (e.g., `organizations.Tag` instead of `*organizations.Tag`)

### Account Controller v2
- Uses AWS SDK v2 client interface throughout
- Context-aware method signatures
- Proper AWS SDK v2 error handling
- Fixed type usage for AWS SDK v2 organizations types

## Notes

- The migration maintains backward compatibility by keeping AWS SDK v1 client code intact
- Controllers are being migrated one at a time to minimize risk
- All AWS SDK v2 client calls use correct types and constants
- Error handling has been updated to use AWS SDK v2 error types

## Testing

- Account controller v2 is ready for testing with AWS SDK v2 client interface
- Need to implement remaining stubbed methods before full testing
- Integration tests will need to be updated to use AWS SDK v2 clients 