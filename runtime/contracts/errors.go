package contracts

import (
	"errors"
	"fmt"
)

type ErrorKind string

const (
	ErrDuplicateHandler   ErrorKind = "duplicate_handler"
	ErrMissingHandler     ErrorKind = "missing_handler"
	ErrUnsupportedHandler ErrorKind = "unsupported_handler"
	ErrNilHandler         ErrorKind = "nil_handler"
	ErrNoEventRecorder    ErrorKind = "no_event_recorder"
	ErrSubscriberFailed   ErrorKind = "subscriber_failed"
	ErrRoleNotAllowed     ErrorKind = "role_not_allowed"
)

// Error is returned for contract registry and dispatch failures.
type Error struct {
	Kind     ErrorKind
	Contract string
	Message  string
	Cause    error
}

func (err Error) Error() string {
	if err.Message != "" {
		return err.Message
	}
	if err.Contract != "" {
		return fmt.Sprintf("%s: %s", err.Kind, err.Contract)
	}
	return string(err.Kind)
}

func (err Error) Unwrap() error {
	return err.Cause
}

// Is reports whether err or one of its causes is a contract Error with kind.
func Is(err error, kind ErrorKind) bool {
	var contractErr Error
	return errors.As(err, &contractErr) && contractErr.Kind == kind
}

func duplicateHandlerError(kind Kind, contract string) error {
	return Error{Kind: ErrDuplicateHandler, Contract: contract, Message: fmt.Sprintf("%s %s already has a handler", kind, contract)}
}

func missingHandlerError(kind Kind, contract string) error {
	return Error{Kind: ErrMissingHandler, Contract: contract, Message: fmt.Sprintf("%s %s has no registered handler", kind, contract)}
}

func unsupportedHandlerError(kind Kind, contract string) error {
	return Error{Kind: ErrUnsupportedHandler, Contract: contract, Message: fmt.Sprintf("%s %s has an unsupported handler signature", kind, contract)}
}

func roleNotAllowedError(kind Kind, contract string, role Role) error {
	return Error{Kind: ErrRoleNotAllowed, Contract: contract, Message: fmt.Sprintf("%s %s is not available to role %q", kind, contract, role)}
}

func nilHandlerError(kind Kind, contract string) error {
	return Error{Kind: ErrNilHandler, Contract: contract, Message: fmt.Sprintf("%s %s cannot register a nil handler", kind, contract)}
}
