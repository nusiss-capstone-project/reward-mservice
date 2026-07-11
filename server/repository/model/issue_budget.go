package model

import "time"

const (
	IssueBudgetStatusOngoing = "ONGOING"
	IssueBudgetStatusEnded   = "ENDED"
)

type IssueBudget struct {
	ID              int64     `gorm:"primaryKey;autoIncrement"`
	IssueRequestID  int64     `gorm:"not null;uniqueIndex"`
	VoucherType     string    `gorm:"type:varchar(64);not null"`
	Unit            string    `gorm:"type:varchar(32);not null"`
	AvailableAmount string    `gorm:"type:decimal(20,8);not null;default:0"`
	TotalAmount     string    `gorm:"type:decimal(20,8);not null;default:0"`
	IssuedAmount    string    `gorm:"type:decimal(20,8);not null;default:0"`
	RefundAmount    string    `gorm:"type:decimal(20,8);not null;default:0"`
	Status          string    `gorm:"type:varchar(32);not null;default:ONGOING"`
	CreatedAt       time.Time `gorm:"autoCreateTime"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

func (IssueBudget) TableName() string {
	return "issue_budget"
}
