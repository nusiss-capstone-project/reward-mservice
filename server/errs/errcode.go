package errs

import "fmt"

const (
	CodeOK = 0

	CodeInvalidRequest          = 40001
	CodeInvalidStatusTransition = 40002
	CodeInvalidPayAddress       = 40003
	CodeInvalidPagination       = 40004
	CodeFinanceDocProjectExists = 40005
	CodeDuplicateBudgetPair     = 40006
	CodePaymentExceedsDocAmount = 40007
	CodeFinanceDocNotApproved   = 40008
	CodeInsufficientAvailable   = 40009
	CodeInsufficientWithhold    = 40010
	CodeDuplicateClientRefID    = 40011

	CodeProjectNotFound        = 40401
	CodeFinanceDocNotFound     = 40402
	CodeProjectBudgetNotFound  = 40403
	CodeFinancePaymentNotFound = 40404
	CodeIssueRequestNotFound   = 40405

	CodeInternalError = 50001
)

const (
	MsgRequestRequired                   = "request is required"
	MsgDocIDRequired                     = "doc_id is required"
	MsgStatusRequired                    = "status is required"
	MsgInvalidAmount                     = "invalid amount"
	MsgOnlyDraftOrRejectedToToApprove    = "only DRAFT or REJECTED can move to TO_APPROVE"
	MsgOnlyToApproveToApprovedOrRejected = "only TO_APPROVE can move to APPROVED or REJECTED"
	MsgUnsupportedTargetStatus           = "unsupported target status"
)

const (
	LogInputError      = "input error"
	LogOperationFailed = "operation failed"
)

var messages = map[int]string{
	CodeOK: "ok",

	CodeInvalidRequest:          "invalid request",
	CodeInvalidStatusTransition: "invalid status transition",
	CodeInvalidPayAddress:       "invalid pay address",
	CodeInvalidPagination:       "invalid pagination parameters",
	CodeFinanceDocProjectExists: "finance doc already exists for project",
	CodeDuplicateBudgetPair:     "duplicate voucher_type and unit in application_detail",
	CodePaymentExceedsDocAmount: "payment amount exceeds doc budget",
	CodeFinanceDocNotApproved:   "finance doc is not approved",
	CodeInsufficientAvailable:   "insufficient available amount",
	CodeInsufficientWithhold:    "insufficient withhold amount",
	CodeDuplicateClientRefID:    "duplicate client_ref_id",

	CodeProjectNotFound:        "project not found",
	CodeFinanceDocNotFound:     "finance doc not found",
	CodeProjectBudgetNotFound:  "project budget not found",
	CodeFinancePaymentNotFound: "finance payment not found",
	CodeIssueRequestNotFound:   "issue request not found",

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
