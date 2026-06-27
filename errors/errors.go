package errors

import (
	// Go internal packages
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
)

// Error defines a standard application error.
type Error struct {
	// Error classification for the application.
	Kind Kind `json:"kind"`

	// Human-readable message.
	Message string `json:"message"`

	// Wrapped underlying error.
	WrappedErr error `json:"wrapped_err,omitempty"`
}

// Error returns the string representation of the error message.
func (e *Error) Error() string {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(e)
	return buf.String()
}

// Unwrap returns the wrapped error.
func (e *Error) Unwrap() error {
	return e.WrappedErr
}

// NewError returns standard go error with given string
func NewError(e string) error {
	return errors.New(e)
}

// Kind defines the kind or class of an error.
type Kind uint8

// Transport agnostic error "kinds"
const (
	Other            Kind = iota // Unclassified error
	Internal                     // Internal error
	Conflict                     // Conflict when an entity already exists
	Invalid                      // Invalid input, validation error etc
	NotFound                     // Entity does not exist
	Unauthorized                 // Unauthorized access
	Forbidden                    // Forbidden access
	MethodNotAllowed             // Method not allowed
	External                     // External service error

	InvalidEnumValue // Invalid enum value
)

func (k Kind) String() string {
	switch k {
	case Other:
		return "unclassified error"
	case Internal:
		return "internal error"
	case Invalid:
		return "invalid input"
	case NotFound:
		return "entity not found"
	case InvalidEnumValue:
		return "invalid enum value"
	case MethodNotAllowed:
		return "method not allowed"
	case External:
		return "external service error"
	default:
		return "unknown error kind"
	}
}

func (k Kind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// E is a helper function which constructs an `*Error`
// You can pass it Kind, error (Err) or string (Message) in any order and it'll construct it.
func E(args ...interface{}) error {
	e := &Error{}
	for _, arg := range args {
		switch arg := arg.(type) {
		case Kind:
			e.Kind = arg
		case error:
			e.WrappedErr = arg
		case string:
			e.Message = arg
		}
	}
	return e
}

var (
	As  = errors.As
	Is  = errors.Is
	New = errors.New
)

type APIError struct {
	Message string `json:"message"`
	// You can add more fields if the API provides them, e.g., Code string `json:"code"`
	ValidationErrors ValidationErrors `json:"validation_errors"`
}

func TranslateErrorToHanlderResponse(err error) (any, int, error) {
	if err != nil {
		switch err.(type) {
		case ValidationErrors:
			return nil, http.StatusBadRequest, E(Invalid, err, "api validation errors")
		case *Error:
			return nil, http.StatusInternalServerError, err
		default:
			return nil, http.StatusInternalServerError, E(Internal, err.Error())
		}
	}
	return nil, http.StatusOK, nil
}
