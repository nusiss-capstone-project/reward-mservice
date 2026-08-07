package data

type IssueRecordVO struct {
	VoucherID     string `json:"voucher_id"`
	VoucherType   string `json:"voucher_type"`
	Unit          string `json:"unit"`
	RewardAmount  string `json:"reward_amount"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

type BudgetVO struct {
	VoucherType     string `json:"voucher_type"`
	Unit            string `json:"unit"`
	AvailableAmount string `json:"available_amount"`
	TotalAmount     string `json:"total_amount"`
	IssuedAmount    string `json:"issued_amount"`
}

type FinanceDocListQuery struct {
	PageQuery
	Status string `form:"status"`
}

type IssueRequestListQuery struct {
	PageQuery
	Status string `form:"status"`
}
