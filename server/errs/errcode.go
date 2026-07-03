package errs

import "fmt"

const (
	CodeOK = 0

	CodeInvalidRequest          = 40001
	CodeInvalidStatusTransition = 40002
	CodeInvalidPayAddress       = 40003
	CodeInvalidPagination       = 40004

	CodeProjectNotFound    = 40401
	CodeFinanceDocNotFound = 40402

	CodeInternalError = 50001
)

var messages = map[int]string{
	CodeOK: "ok",

	CodeInvalidRequest:          "invalid request",
	CodeInvalidStatusTransition: "invalid status transition",
	CodeInvalidPayAddress:       "invalid pay address",
	CodeInvalidPagination:       "invalid pagination parameters",

	CodeProjectNotFound:    "project not found",
	CodeFinanceDocNotFound: "finance doc not found",

	CodeInternalError: "internal server error",
}

func Message(code int) string {
	if msg, ok := messages[code]; ok {
		return msg
	}
	return messages[CodeInternalError]
}

type AppError struct {
	Code    int
	Message string
}

func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return Message(e.Code)
}

func New(code int, message string) *AppError {
	if message == "" {
		message = Message(code)
	}
	return &AppError{Code: code, Message: message}
}

func Wrap(code int, err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if As(err, &appErr) {
		return appErr
	}
	return New(code, fmt.Sprintf("%s: %v", Message(code), err))
}

func As(err error, target **AppError) bool {
	if err == nil {
		return false
	}
	appErr, ok := err.(*AppError)
	if !ok {
		return false
	}
	*target = appErr
	return true
}
