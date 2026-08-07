package model

import (
	"time"

	"gorm.io/datatypes"
)

const (
	TemplateStatusDraft     = "DRAFT"
	TemplateStatusPublished = "PUBLISHED"

	TemplateTypeFixed   = "FIXED"
	TemplateTypeDynamic = "DYNAMIC"
)

type FixTemplateConfig struct {
	Amount string `json:"amount"`
}

type DynamicTemplateConfig struct {
	BaseMetric string  `json:"base_metric"`
	Rate       float64 `json:"rate"`
	Cap        string  `json:"cap,omitempty"`
}

type Template struct {
	ID          int64          `gorm:"primaryKey;autoIncrement"`
	Title       string         `gorm:"type:varchar(128);not null;default:''"`
	VoucherType string         `gorm:"type:varchar(64);not null"`
	Unit        string         `gorm:"type:varchar(32);not null"`
	Type        string         `gorm:"type:varchar(32);not null"`
	Config      datatypes.JSON `gorm:"type:json;not null"`
	Status      string         `gorm:"type:varchar(32);not null;default:DRAFT"`
	CreatedAt   time.Time      `gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
}

func (Template) TableName() string {
	return "templates"
}
