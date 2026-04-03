package apperrors

import "fmt"

const (
	CategoryUser   = "user"
	CategorySystem = "system"
)

// Error carries structured metadata that frontend can inspect.
type Error struct {
	Code     string `json:"code"`
	Category string `json:"category"`
	Message  string `json:"message"`
	Cause    error  `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func User(code, message string) error {
	return &Error{Code: code, Category: CategoryUser, Message: message}
}

func UserWrap(code, message string, cause error) error {
	return &Error{Code: code, Category: CategoryUser, Message: message, Cause: cause}
}

func System(code, message string, cause error) error {
	return &Error{Code: code, Category: CategorySystem, Message: message, Cause: cause}
}
