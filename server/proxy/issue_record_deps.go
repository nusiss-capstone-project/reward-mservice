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
	Issue(ctx context.Context, record *IssueRecordSnapshot, amount string) (businessSuccess bool, failedReason string, err error)
}

func GetVoucherIssuer() VoucherIssuer {
	return &noopVoucherIssuer{}
}

type IssueRecordSnapshot struct {
	VoucherID   string
	UserID      int64
	ProjectID   int64
	VoucherType string
	Unit        string
}

type noopVoucherIssuer struct{}

func (noopVoucherIssuer) Issue(context.Context, *IssueRecordSnapshot, string) (bool, string, error) {
	return true, "", nil
}
