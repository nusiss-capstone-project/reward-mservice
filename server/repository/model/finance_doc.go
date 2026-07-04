package model

import (
	"time"

	"gorm.io/datatypes"
)

const (
	FinanceDocStatusDraft     = "DRAFT"
	FinanceDocStatusToApprove = "TO_APPROVE"
	FinanceDocStatusApproved  = "APPROVED"
	FinanceDocStatusRejected  = "REJECTED"
)

type ApplicationDetailItem struct {
	PayAddress string `json:"pay_address"`
	Amount     string `json:"amount"`
}

type FinanceDoc struct {
	DocID             string         `gorm:"primaryKey;type:varchar(64)"`
	ProjectID         int64          `gorm:"not null;uniqueIndex"`
	Description       string         `gorm:"type:text"`
	ApplicationDetail datatypes.JSON `gorm:"type:json;not null"`
	Creator           string         `gorm:"type:varchar(128);not null"`
	Status            string         `gorm:"type:varchar(32);not null;default:DRAFT"`
	Remark            string         `gorm:"type:varchar(512)"`
	CreatedAt         time.Time      `gorm:"autoCreateTime"`
	UpdatedAt         time.Time      `gorm:"autoUpdateTime"`
}

func (FinanceDoc) TableName() string {
	return "finance_docs"
}
