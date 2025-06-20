// Code generated by smithy-go-codegen DO NOT EDIT.

package types

import (
	"fmt"
	smithy "github.com/aws/smithy-go"
)

// The requested bucket name is not available. The bucket namespace is shared by
// all users of the system. Select a different name and try again.
type BucketAlreadyExists struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *BucketAlreadyExists) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *BucketAlreadyExists) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *BucketAlreadyExists) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "BucketAlreadyExists"
	}
	return *e.ErrorCodeOverride
}
func (e *BucketAlreadyExists) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// The bucket you tried to create already exists, and you own it. Amazon S3
// returns this error in all Amazon Web Services Regions except in the North
// Virginia Region. For legacy compatibility, if you re-create an existing bucket
// that you already own in the North Virginia Region, Amazon S3 returns 200 OK and
// resets the bucket access control lists (ACLs).
type BucketAlreadyOwnedByYou struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *BucketAlreadyOwnedByYou) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *BucketAlreadyOwnedByYou) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *BucketAlreadyOwnedByYou) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "BucketAlreadyOwnedByYou"
	}
	return *e.ErrorCodeOverride
}
func (e *BucketAlreadyOwnedByYou) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

//	The existing object was created with a different encryption type. Subsequent
//
// write requests must include the appropriate encryption parameters in the request
// or while creating the session.
type EncryptionTypeMismatch struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *EncryptionTypeMismatch) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *EncryptionTypeMismatch) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *EncryptionTypeMismatch) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "EncryptionTypeMismatch"
	}
	return *e.ErrorCodeOverride
}
func (e *EncryptionTypeMismatch) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// Parameters on this idempotent request are inconsistent with parameters used in
// previous request(s).
//
// For a list of error codes and more information on Amazon S3 errors, see [Error codes].
//
// Idempotency ensures that an API request completes no more than one time. With
// an idempotent request, if the original request completes successfully, any
// subsequent retries complete successfully without performing any further actions.
//
// [Error codes]: https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html#ErrorCodeList
type IdempotencyParameterMismatch struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *IdempotencyParameterMismatch) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *IdempotencyParameterMismatch) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *IdempotencyParameterMismatch) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "IdempotencyParameterMismatch"
	}
	return *e.ErrorCodeOverride
}
func (e *IdempotencyParameterMismatch) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// Object is archived and inaccessible until restored.
//
// If the object you are retrieving is stored in the S3 Glacier Flexible Retrieval
// storage class, the S3 Glacier Deep Archive storage class, the S3
// Intelligent-Tiering Archive Access tier, or the S3 Intelligent-Tiering Deep
// Archive Access tier, before you can retrieve the object you must first restore a
// copy using [RestoreObject]. Otherwise, this operation returns an InvalidObjectState error. For
// information about restoring archived objects, see [Restoring Archived Objects]in the Amazon S3 User Guide.
//
// [RestoreObject]: https://docs.aws.amazon.com/AmazonS3/latest/API/API_RestoreObject.html
// [Restoring Archived Objects]: https://docs.aws.amazon.com/AmazonS3/latest/dev/restoring-objects.html
type InvalidObjectState struct {
	Message *string

	ErrorCodeOverride *string

	StorageClass StorageClass
	AccessTier   IntelligentTieringAccessTier

	noSmithyDocumentSerde
}

func (e *InvalidObjectState) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *InvalidObjectState) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *InvalidObjectState) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "InvalidObjectState"
	}
	return *e.ErrorCodeOverride
}
func (e *InvalidObjectState) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// You may receive this error in multiple cases. Depending on the reason for the
// error, you may receive one of the messages below:
//
//   - Cannot specify both a write offset value and user-defined object metadata
//     for existing objects.
//
//   - Checksum Type mismatch occurred, expected checksum Type: sha1, actual
//     checksum Type: crc32c.
//
//   - Request body cannot be empty when 'write offset' is specified.
type InvalidRequest struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *InvalidRequest) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *InvalidRequest) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *InvalidRequest) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "InvalidRequest"
	}
	return *e.ErrorCodeOverride
}
func (e *InvalidRequest) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

//	The write offset value that you specified does not match the current object
//
// size.
type InvalidWriteOffset struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *InvalidWriteOffset) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *InvalidWriteOffset) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *InvalidWriteOffset) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "InvalidWriteOffset"
	}
	return *e.ErrorCodeOverride
}
func (e *InvalidWriteOffset) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// The specified bucket does not exist.
type NoSuchBucket struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *NoSuchBucket) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *NoSuchBucket) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *NoSuchBucket) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "NoSuchBucket"
	}
	return *e.ErrorCodeOverride
}
func (e *NoSuchBucket) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// The specified key does not exist.
type NoSuchKey struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *NoSuchKey) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *NoSuchKey) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *NoSuchKey) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "NoSuchKey"
	}
	return *e.ErrorCodeOverride
}
func (e *NoSuchKey) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// The specified multipart upload does not exist.
type NoSuchUpload struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *NoSuchUpload) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *NoSuchUpload) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *NoSuchUpload) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "NoSuchUpload"
	}
	return *e.ErrorCodeOverride
}
func (e *NoSuchUpload) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// The specified content does not exist.
type NotFound struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *NotFound) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *NotFound) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *NotFound) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "NotFound"
	}
	return *e.ErrorCodeOverride
}
func (e *NotFound) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// This action is not allowed against this storage tier.
type ObjectAlreadyInActiveTierError struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *ObjectAlreadyInActiveTierError) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *ObjectAlreadyInActiveTierError) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *ObjectAlreadyInActiveTierError) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "ObjectAlreadyInActiveTierError"
	}
	return *e.ErrorCodeOverride
}
func (e *ObjectAlreadyInActiveTierError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// The source object of the COPY action is not in the active tier and is only
// stored in Amazon S3 Glacier.
type ObjectNotInActiveTierError struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *ObjectNotInActiveTierError) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *ObjectNotInActiveTierError) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *ObjectNotInActiveTierError) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "ObjectNotInActiveTierError"
	}
	return *e.ErrorCodeOverride
}
func (e *ObjectNotInActiveTierError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

//	You have attempted to add more parts than the maximum of 10000 that are
//
// allowed for this object. You can use the CopyObject operation to copy this
// object to another and then add more data to the newly copied object.
type TooManyParts struct {
	Message *string

	ErrorCodeOverride *string

	noSmithyDocumentSerde
}

func (e *TooManyParts) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode(), e.ErrorMessage())
}
func (e *TooManyParts) ErrorMessage() string {
	if e.Message == nil {
		return ""
	}
	return *e.Message
}
func (e *TooManyParts) ErrorCode() string {
	if e == nil || e.ErrorCodeOverride == nil {
		return "TooManyParts"
	}
	return *e.ErrorCodeOverride
}
func (e *TooManyParts) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }
