package errs

import "errors"

var (
	ErrExpression          = errors.New("incorrect expression entered")
	ErrHeaderAuthorization = errors.New("invalid header Authorization")
	ErrServer              = errors.New("server error")
	ErrRequestJSON         = errors.New("request with invalid json")
	ErrExpressionId        = errors.New("invalid id of expression")
	ErrTokenExpired        = errors.New("the validity period of the jwt token has expired")
)
