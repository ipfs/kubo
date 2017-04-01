package e

import (
	"fmt"
	"runtime/debug"
)

// TypeErr returns an error with a string that explains what error was expected and what was received.
func TypeErr(expected, actual interface{}) error {
	return fmt.Errorf("expected type %T, got %T", expected, actual)
}

// compile time type check that HandlerError is an error
var _ error = New(nil)

// HandlerError is adds a stack trace to an error
type HandlerError struct {
	Err   error
	Stack []byte
}

// Error makes HandlerError implement error
func (err HandlerError) Error() string {
	return fmt.Sprintf("%s in:\n%s", err.Err.Error(), err.Stack)
}

// New returns a new HandlerError
func New(err error) HandlerError {
	return HandlerError{Err: err, Stack: debug.Stack()}
}
