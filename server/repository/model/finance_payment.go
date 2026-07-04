package model

import "time"

const FinancePaymentStatusPaid = "PAID"

type FinancePayment struct {
	FinanceDocID     string    `gorm:"type:varchar(64);not null;index"`
	PaymentID        string    `gorm:"primaryKey;type:varchar(64)"`
	PaymentAddress   string    `gorm:"type:varchar(128);not null"`
	Amount           string    `gorm:"type:decimal(20,8);not null"`
	PaymentStatus    string    `gorm:"type:varchar(32);not null"`
	OffsettedAmount  string    `gorm:"type:decimal(20,8);not null;default:0"`
	OffsettingAmount string    `gorm:"type:decimal(20,8);not null;default:0"`
	RefundedAmount   string    `gorm:"type:decimal(20,8);not null;default:0"`
	RefundingAmount  string    `gorm:"type:decimal(20,8);not null;default:0"`
	CreatedAt        time.Time `gorm:"autoCreateTime"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime"`
}

func (FinancePayment) TableName() string {
	return "finance_payments"
}
