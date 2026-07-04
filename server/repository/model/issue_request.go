package model

import "time"

const (
	IssueRequestStatusDraft     = "DRAFT"
	IssueRequestStatusToApprove = "TO_APPROVE"
	IssueRequestStatusApproved  = "APPROVED"
	IssueRequestStatusRejected  = "REJECTED"
	IssueRequestStatusOngoing   = "ONGOING"

	IssueRequestExpenseTypeRefund = "REFUND"
	IssueRequestExpenseTypeReward = "REWARD"
	IssueRequestExpenseTypeOthers = "OTHERS"
)

type IssueRequest struct {
	ID            int64     `gorm:"primaryKey;autoIncrement"`
	ProjectID     int64     `gorm:"not null;index"`
	VoucherType   string    `gorm:"type:varchar(64);not null"`
	Unit          string    `gorm:"type:varchar(32);not null"`
	Amount        string    `gorm:"type:decimal(20,8);not null"`
	RequestStatus string    `gorm:"type:varchar(32);not null;default:DRAFT"`
	ExpenseType   string    `gorm:"type:varchar(32);not null"`
	Creator       string    `gorm:"type:varchar(128);not null"`
	Remark        string    `gorm:"type:varchar(512);not null;default:''"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

func (IssueRequest) TableName() string {
	return "issue_requests"
}
