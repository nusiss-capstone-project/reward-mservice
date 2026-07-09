package model

import "time"

const (
	RewardRequestStatusPending   = "PENDING"
	RewardRequestStatusCompleted = "COMPLETED"
)

type RewardRequest struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	ClientRefID  string    `gorm:"type:varchar(128);not null;uniqueIndex"`
	UserID       int64     `gorm:"not null"`
	ProjectID    int64     `gorm:"not null;index"`
	VoucherType  string    `gorm:"type:varchar(64);not null"`
	Unit         string    `gorm:"type:varchar(32);not null"`
	Amount       string    `gorm:"type:decimal(20,8);not null"`
	Status       string    `gorm:"type:varchar(32);not null;default:PENDING;index"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

func (RewardRequest) TableName() string {
	return "reward_requests"
}
