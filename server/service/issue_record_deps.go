package service

import (
	"context"
	"errors"
)

var (
	ErrDistributionDeferred = errors.New("distribution deferred")
	ErrDistributionRetry    = errors.New("distribution retry")
)

const (
	distributionResultStatusDistributed = "DISTRIBUTED"
	distributionResultStatusFailed      = "FAILED"
)

type RiskChecker interface {
	Check(ctx context.Context, userID int64) (passed bool, reason string)
}

type noopRiskChecker struct{}

func (noopRiskChecker) Check(context.Context, int64) (bool, string) {
	return true, ""
}

type VoucherIssuer interface {
	Issue(ctx context.Context, record *issueRecordSnapshot, amount string) (businessSuccess bool, failedReason string, err error)
}

type issueRecordSnapshot struct {
	VoucherID   string
	UserID      int64
	ProjectID   int64
	VoucherType string
	Unit        string
}

type noopVoucherIssuer struct{}

func (noopVoucherIssuer) Issue(context.Context, *issueRecordSnapshot, string) (bool, string, error) {
	return true, "", nil
}
