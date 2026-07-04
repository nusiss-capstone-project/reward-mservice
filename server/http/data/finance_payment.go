package data

type CreateFinancePaymentRequest struct {
	PaymentAddress string `json:"payment_address" binding:"required"`
	Amount         string `json:"amount" binding:"required"`
	Unit           string `json:"unit" binding:"required"`
}

type FinancePaymentVO struct {
	FinanceDocID     string `json:"finance_doc_id"`
	PaymentID        string `json:"payment_id"`
	PaymentAddress   string `json:"payment_address"`
	Amount           string `json:"amount"`
	PaymentStatus    string `json:"payment_status"`
	OffsettedAmount  string `json:"offsetted_amount"`
	OffsettingAmount string `json:"offsetting_amount"`
	RefundedAmount   string `json:"refunded_amount"`
	RefundingAmount  string `json:"refunding_amount"`
	CreatedAt        string `json:"created_at,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}
