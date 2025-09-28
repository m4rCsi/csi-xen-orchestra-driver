// Copyright 2025 Marc Siegenthaler
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xoa

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sourcegraph/jsonrpc2"
)

var (
	// Connection errors
	ErrConnectionError      = errors.New("connection error")
	ErrInvalidArgument      = errors.New("invalid argument")
	ErrUnmarshalError       = errors.New("unmarshalling error")
	ErrNotImplemented       = errors.New("not implemented (client)")
	ErrMultipleObjectsFound = errors.New("multiple objects found")
	ErrObjectNotFound       = errors.New("object not found (after filtering)")

	// Unknown error
	ErrUnknownError = errors.New("unknown error")
)

// ConvertJSONRPCError converts an JSONRPCError to a specific error type based on the code
func ConvertJSONRPCError(apiErr *jsonrpc2.Error) error {
	if specificErr, exists := errorCodeMap[apiErr.Code]; exists {
		return fmt.Errorf("%w: %s", specificErr, apiErr.Message)
	}

	// -32000 is the code for unknown errors
	if apiErr.Code == -32000 {
		if stringcode, _, ok := strings.Cut(apiErr.Message, "("); ok {
			if specificErr, exists := unknownErrorStringCodeMap[stringcode]; exists {
				return fmt.Errorf("%w: %s", specificErr, apiErr.Message)
			}
		}
	}

	// For unknown error codes, return a generic unknown error
	return fmt.Errorf("%w: code: [%d], message: [%s]", ErrUnknownError, apiErr.Code, apiErr.Message)
}

// From: https://github.com/vatesfr/xen-orchestra/blob/2effd8520ad561d8f4df9cf985257030133fd330/packages/xo-common/api-errors.js

var (
	// General errors
	ErrNotImplementedOnServer = errors.New("not implemented (server)")
	ErrNoSuchObject           = errors.New("no such object")
	ErrUnauthorized           = errors.New("not enough permissions")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrForbiddenOperation     = errors.New("forbidden operation")
	ErrNoHostsAvailable       = errors.New("no hosts available")
	ErrAuthenticationFailed   = errors.New("authentication failed")
	ErrServerUnreachable      = errors.New("server unreachable")
	ErrInvalidParameters      = errors.New("invalid parameters")

	// VM-related errors
	ErrVMMissingPvDrivers = errors.New("missing PV drivers")
	ErrVMIsTemplate       = errors.New("VM is a template")
	ErrVMBadPowerState    = errors.New("VM state is incorrect")
	ErrVMLacksFeature     = errors.New("VM lacks required feature")

	// System errors
	ErrNotSupportedDuringUpgrade = errors.New("not supported during upgrade")
	ErrObjectAlreadyExists       = errors.New("object already exists")
	ErrVDIInUse                  = errors.New("VDI in use")
	ErrHostOffline               = errors.New("host offline")
	ErrOperationBlocked          = errors.New("operation blocked")
	ErrPatchPrecheckFailed       = errors.New("patch precheck failed")
	ErrOperationFailed           = errors.New("operation failed")

	// Audit errors
	ErrMissingAuditRecord = errors.New("missing audit record")
	ErrAlteredAuditRecord = errors.New("altered audit record")

	// Resource errors
	ErrNotEnoughResources  = errors.New("not enough resources in resource set")
	ErrIncorrectState      = errors.New("incorrect state")
	ErrFeatureUnauthorized = errors.New("feature unauthorized")

	// "Unknown" Errors
	// These errors don't have a code, but we can try and detect them by the message
	ErrOtherOperationInProgress = errors.New("other operation in progress")
	ErrOperationNotAllowed      = errors.New("operation not allowed")
	ErrDeviceAlreadyExists      = errors.New("device already exists")
)

// errorCodeMap maps error codes to their corresponding error types
var errorCodeMap = map[int64]error{
	// General errors
	0:  ErrNotImplementedOnServer,
	1:  ErrNoSuchObject,
	2:  ErrUnauthorized,
	3:  ErrInvalidCredentials,
	5:  ErrForbiddenOperation,
	7:  ErrNoHostsAvailable,
	8:  ErrAuthenticationFailed,
	9:  ErrServerUnreachable,
	10: ErrInvalidParameters,

	// VM-related errors
	11: ErrVMMissingPvDrivers,
	12: ErrVMIsTemplate,
	13: ErrVMBadPowerState,
	14: ErrVMLacksFeature,

	// System errors
	15: ErrNotSupportedDuringUpgrade,
	16: ErrObjectAlreadyExists,
	17: ErrVDIInUse,
	18: ErrHostOffline,
	19: ErrOperationBlocked,
	20: ErrPatchPrecheckFailed,
	21: ErrOperationFailed,

	// Audit errors
	22: ErrMissingAuditRecord,
	23: ErrAlteredAuditRecord,

	// Resource errors
	24: ErrNotEnoughResources,
	25: ErrIncorrectState,
	26: ErrFeatureUnauthorized,
}

var unknownErrorStringCodeMap = map[string]error{
	"OTHER_OPERATION_IN_PROGRESS": ErrOtherOperationInProgress,
	"OPERATION_NOT_ALLOWED":       ErrOperationNotAllowed,
	"DEVICE_ALREADY_EXISTS":       ErrDeviceAlreadyExists,
}
