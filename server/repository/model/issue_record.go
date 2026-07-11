package model

import "time"

const (
	IssueRecordStatusPending = "PENDING"
	IssueRecordStatusIssued  = "ISSUED"
	IssueRecordStatusFailed  = "FAILED"
)

type IssueRecord struct {
	ID                int64     `gorm:"primaryKey;autoIncrement"`
	VoucherID         string    `gorm:"type:varchar(128);not null;uniqueIndex"`
	RewardRequestID   int64     `gorm:"not null;uniqueIndex"`
	IssueRequestID    *int64    `gorm:"index"`
	ProjectID         int64     `gorm:"not null;index"`
	UserID            int64     `gorm:"not null"`
	VoucherType       string    `gorm:"type:varchar(64);not null"`
	Unit              string    `gorm:"type:varchar(32);not null"`
	RewardAmount      string    `gorm:"type:decimal(20,8);not null;default:0"`
	IssueStatus       string    `gorm:"type:varchar(32);not null"`
	OffsetStatus      string    `gorm:"type:varchar(32);not null;default:''"`
	Reason            string    `gorm:"type:varchar(512);not null;default:''"`
	ClientReferenceID string    `gorm:"type:varchar(128);not null;index"`
	CreatedAt         time.Time `gorm:"autoCreateTime"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime"`
}

func (IssueRecord) TableName() string {
	return "issue_records"
}
