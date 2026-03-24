package sshops

import (
	"errors"
	"fmt"
)

type UserError struct {
	Code    string
	Message string
	Err     error
}

func NewUserError(code, message string, err error) error {
	return &UserError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

func (e *UserError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Message
	}
	if e.Message == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *UserError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ErrorCode(err error) string {
	var userErr *UserError
	if errors.As(err, &userErr) && userErr.Code != "" {
		return userErr.Code
	}
	return "internal_error"
}

func ErrorMessage(err error) string {
	var userErr *UserError
	if errors.As(err, &userErr) && userErr.Message != "" {
		return userErr.Message
	}
	if err == nil {
		return ""
	}
	return err.Error()
}
