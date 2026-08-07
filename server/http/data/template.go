package data

import "encoding/json"

type FixTemplateConfigVO struct {
	Amount string `json:"amount" example:"100.00"`
}

type DynamicTemplateConfigVO struct {
	BaseMetric string  `json:"base_metric" example:"net_deposit"`
	Rate       float64 `json:"rate" example:"0.10"`
	Cap        string  `json:"cap,omitempty" example:"100.00"`
}

type CreateTemplateRequest struct {
	Title       string          `json:"title" binding:"required" example:"Welcome Bonus"`
	VoucherType string          `json:"voucher_type" binding:"required" example:"CRYPTO"`
	Unit        string          `json:"unit" binding:"required" example:"CRYPTO_USDT"`
	Type        string          `json:"type" binding:"required" enums:"FIXED,DYNAMIC" example:"FIXED"`
	Config      json.RawMessage `json:"config" binding:"required" swaggertype:"string" example:"{\"amount\":\"100.00\"}"`
}

type CreateTemplateResponse struct {
	TemplateID int64 `json:"template_id"`
}

type UpdateTemplateRequest struct {
	Title  string          `json:"title" example:"Welcome Bonus"`
	Config json.RawMessage `json:"config" binding:"required" swaggertype:"string" example:"{\"amount\":\"100.00\"}"`
}

type PublishTemplateResponse struct {
	TemplateID int64  `json:"template_id"`
	Status     string `json:"status"`
}

type TemplateVO struct {
	ID          int64       `json:"id" example:"1"`
	Title       string      `json:"title" example:"Welcome Bonus"`
	VoucherType string      `json:"voucher_type" example:"CRYPTO"`
	Unit        string      `json:"unit" example:"CRYPTO_USDT"`
	Type        string      `json:"type" enums:"FIXED,DYNAMIC" example:"FIXED"`
	Config      interface{} `json:"config"`
	Status      string      `json:"status" enums:"DRAFT,PUBLISHED" example:"DRAFT"`
	CreatedAt   string      `json:"created_at,omitempty" example:"2026-07-12 10:00:00"`
	UpdatedAt   string      `json:"updated_at,omitempty" example:"2026-07-12 10:00:00"`
}

type TemplateListQuery struct {
	PageQuery
	Status string `form:"status" enums:"DRAFT,PUBLISHED" example:"DRAFT"`
}
