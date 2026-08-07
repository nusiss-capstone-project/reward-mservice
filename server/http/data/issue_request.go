package data

type CreateIssueRequestRequest struct {
	VoucherType string `json:"voucher_type" binding:"required"`
	Unit        string `json:"unit" binding:"required"`
	Amount      string `json:"amount" binding:"required"`
	ExpenseType string `json:"expense_type" binding:"required,oneof=REFUND REWARD OTHERS"`
	Remark      string `json:"remark"`
}

type UpdateIssueRequestRequest struct {
	VoucherType string `json:"voucher_type" binding:"required"`
	Unit        string `json:"unit" binding:"required"`
	Amount      string `json:"amount" binding:"required"`
	Remark      string `json:"remark"`
}

type SubmitIssueRequestRequest struct {
	Status string `json:"status" binding:"required,oneof=TO_APPROVE"`
	Remark string `json:"remark"`
}

type ApproveIssueRequestRequest struct {
	Status string `json:"status" binding:"required,oneof=APPROVED REJECTED"`
	Remark string `json:"remark"`
}

type IssueRequestVO struct {
	ID            int64  `json:"id"`
	ProjectID     int64  `json:"project_id"`
	VoucherType   string `json:"voucher_type"`
	Unit          string `json:"unit"`
	Amount        string `json:"amount"`
	RequestStatus string `json:"request_status"`
	ExpenseType   string `json:"expense_type"`
	Remark        string `json:"remark"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

type UpdateIssueRequestResponse struct {
	ID            int64  `json:"id"`
	RequestStatus string `json:"request_status"`
	Remark        string `json:"remark"`
}
