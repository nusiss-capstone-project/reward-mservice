package data

type ApplicationDetailItemVO struct {
	PayAddress string `json:"pay_address" binding:"required"`
	Amount     string `json:"amount" binding:"required"`
}

type CreateFinanceDocRequest struct {
	ProjectID         int64                     `json:"project_id" binding:"required"`
	Description       string                    `json:"description"`
	ApplicationDetail []ApplicationDetailItemVO `json:"application_detail" binding:"required,min=1,dive"`
}

type CreateFinanceDocResponse struct {
	DocID string `json:"doc_id"`
}

type FinanceDocVO struct {
	DocID             string                    `json:"doc_id"`
	ProjectID         int64                     `json:"project_id"`
	Project           *ProjectVO                `json:"project,omitempty"`
	Description       string                    `json:"description"`
	ApplicationDetail []ApplicationDetailItemVO `json:"application_detail"`
	Status            string                    `json:"status"`
	Remark            string                    `json:"remark"`
	CreatedAt         string                    `json:"created_at,omitempty"`
	UpdatedAt         string                    `json:"updated_at,omitempty"`
}

type SubmitFinanceDocRequest struct {
	Remark string `json:"remark"`
}

type ApproveFinanceDocRequest struct {
	Status string `json:"status" binding:"required,oneof=APPROVED REJECTED"`
	Remark string `json:"remark"`
}

type UpdateFinanceDocContentRequest struct {
	Description       string                    `json:"description"`
	ApplicationDetail []ApplicationDetailItemVO `json:"application_detail" binding:"required,min=1,dive"`
}

type UpdateFinanceDocRequest struct {
	Status string `json:"status" binding:"required"`
	Remark string `json:"remark"`
}

type UpdateFinanceDocResponse struct {
	DocID  string `json:"doc_id"`
	Status string `json:"status"`
	Remark string `json:"remark"`
}
