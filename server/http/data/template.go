package data

import "encoding/json"

type TemplateConfigBase struct{}

type FixTemplateConfigVO struct {
	TemplateConfigBase
	Amount string `json:"amount"`
}

type DynamicTemplateConfigVO struct {
	TemplateConfigBase
	BaseMetric string  `json:"base_metric"`
	Rate       float64 `json:"rate"`
	Cap        string  `json:"cap,omitempty"`
}

type CreateTemplateRequest struct {
	VoucherType string          `json:"voucher_type" binding:"required"`
	Unit        string          `json:"unit" binding:"required"`
	Type        string          `json:"type" binding:"required"`
	Config      json.RawMessage `json:"config" binding:"required"`
}

type CreateTemplateResponse struct {
	TemplateID int64 `json:"template_id"`
}

type UpdateTemplateRequest struct {
	Config json.RawMessage `json:"config" binding:"required"`
}

type PublishTemplateResponse struct {
	TemplateID int64  `json:"template_id"`
	Status     string `json:"status"`
}

type TemplateVO struct {
	ID          int64       `json:"id"`
	VoucherType string      `json:"voucher_type"`
	Unit        string      `json:"unit"`
	Type        string      `json:"type"`
	Config      interface{} `json:"config"`
	Status      string      `json:"status"`
	CreatedAt   string      `json:"created_at,omitempty"`
	UpdatedAt   string      `json:"updated_at,omitempty"`
}
