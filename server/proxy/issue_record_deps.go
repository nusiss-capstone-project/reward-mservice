package proxy

import (
	"context"
)

type RiskChecker interface {
	Check(ctx context.Context, userID int64) (passed bool, reason string)
}

func GetRiskChecker() RiskChecker {
	return &noopRiskChecker{}
}

type noopRiskChecker struct{}

func (noopRiskChecker) Check(context.Context, int64) (bool, string) {
	return true, ""
}

type VoucherIssuer interface {
	// Issue performs downstream voucher issuance.
	// On success: businessSuccess=true and issue_record can be updated to ISSUED.
	// On permanent business failure: businessSuccess=false, err=nil (issue_record -> FAILED).
	// On transient failure: err!=nil (issue_record stays PENDING and will retry).
	Issue(ctx context.Context, record *IssueRecordSnapshot, amount string) (businessSuccess bool, failedReason string, err error)
}

type IssueRecordSnapshot struct {
	VoucherID   string
	UserID      int64
	ProjectID   int64
	VoucherType string
	Unit        string
}
