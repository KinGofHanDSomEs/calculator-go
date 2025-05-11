package errs

import "errors"

var (
	ErrRequestJSON        = errors.New("request with invalid json")
	ErrRegisterLogin      = errors.New("request with an empty username")
	ErrRegisterPassword   = errors.New("request with an empty password")
	ErrServer             = errors.New("server error")
	ErrRedundantRecording = errors.New("user with such login or password exists")
	ErrUserNotExist       = errors.New("user with such login and password does not exist")
)
