package model

import "time"

type ProjectBudget struct {
	ID              int64     `gorm:"primaryKey;autoIncrement"`
	FinanceDocID    string    `gorm:"type:varchar(64);not null;uniqueIndex:uk_project_budget_doc"`
	ProjectID       int64     `gorm:"not null;uniqueIndex:uk_project_budget_project"`
	VoucherType     string    `gorm:"type:varchar(64);not null;uniqueIndex:uk_project_budget_doc;uniqueIndex:uk_project_budget_project"`
	Unit            string    `gorm:"type:varchar(32);not null;uniqueIndex:uk_project_budget_doc;uniqueIndex:uk_project_budget_project"`
	TotalAmount     string    `gorm:"type:decimal(20,8);not null;default:0"`
	AvailableAmount string    `gorm:"type:decimal(20,8);not null;default:0"`
	WithholdAmount  string    `gorm:"type:decimal(20,8);not null;default:0"`
	IssuedAmount    string    `gorm:"type:decimal(20,8);not null;default:0"`
	RefundAmount    string    `gorm:"type:decimal(20,8);not null;default:0"`
	CreatedAt       time.Time `gorm:"autoCreateTime"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

func (ProjectBudget) TableName() string {
	return "project_budget"
}
