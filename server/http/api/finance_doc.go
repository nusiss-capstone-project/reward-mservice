package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

// CreateFinanceDoc creates a finance doc in DRAFT status.
//
// @Summary Create finance doc
// @Description Create a finance doc for budget application.
// @Tags Admin-FinanceDoc
// @Accept json
// @Produce json
// @Param body body data.CreateFinanceDocRequest true "Finance doc payload"
// @Success 200 {object} data.BaseResponse{data=data.CreateFinanceDocResponse}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs [post]
func CreateFinanceDoc(c *gin.Context) {
	req := &data.CreateFinanceDocRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{
			Code:   errs.CodeInvalidRequest,
			ErrMsg: err.Error(),
		})
		return
	}

	docID, err := service.GetFinanceDocService().CreateFinanceDoc(c.Request.Context(), req)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, &data.CreateFinanceDocResponse{DocID: docID})
}

// ListFinanceDocs lists finance docs with pagination.
//
// @Summary List finance docs
// @Description List finance docs for admin users.
// @Tags Admin-FinanceDoc
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(20)
// @Success 200 {object} data.BaseResponse{data=data.PageResult}
// @Failure 400 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs [get]
func ListFinanceDocs(c *gin.Context) {
	query := data.PageQuery{}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{
			Code:   errs.CodeInvalidRequest,
			ErrMsg: err.Error(),
		})
		return
	}
	page, size := query.Normalize()

	result, err := service.GetFinanceDocService().ListFinanceDocs(c.Request.Context(), page, size)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// GetFinanceDocDetail returns finance doc detail.
//
// @Summary Get finance doc detail
// @Description Get finance doc detail by doc_id.
// @Tags Admin-FinanceDoc
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Success 200 {object} data.BaseResponse{data=data.FinanceDocVO}
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id} [get]
func GetFinanceDocDetail(c *gin.Context) {
	docID := c.Param("doc_id")
	result, err := service.GetFinanceDocService().GetFinanceDocDetail(c.Request.Context(), docID)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// SubmitFinanceDocForApproval submits a finance doc for approval.
//
// @Summary Submit finance doc for approval
// @Description Move finance doc from DRAFT or REJECTED to TO_APPROVE.
// @Tags Admin-FinanceDoc
// @Accept json
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Param body body data.SubmitFinanceDocRequest false "Submit payload"
// @Success 200 {object} data.BaseResponse{data=data.UpdateFinanceDocResponse}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id}/submission [patch]
func SubmitFinanceDocForApproval(c *gin.Context) {
	req := &data.SubmitFinanceDocRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{
			Code:   errs.CodeInvalidRequest,
			ErrMsg: err.Error(),
		})
		return
	}

	docID := c.Param("doc_id")
	result, err := service.GetFinanceDocService().UpdateFinanceDocStatus(c.Request.Context(), docID, &data.UpdateFinanceDocRequest{
		Status: model.FinanceDocStatusToApprove,
		Remark: req.Remark,
	})
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// ApproveFinanceDoc approves or rejects a finance doc.
//
// @Summary Approve or reject finance doc
// @Description Move finance doc from TO_APPROVE to APPROVED or REJECTED. When approved, a Kafka event is published to initialize project budget.
// @Tags Admin-FinanceDoc
// @Accept json
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Param body body data.ApproveFinanceDocRequest true "Approval payload"
// @Success 200 {object} data.BaseResponse{data=data.UpdateFinanceDocResponse}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id}/approval [patch]
func ApproveFinanceDoc(c *gin.Context) {
	req := &data.ApproveFinanceDocRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{
			Code:   errs.CodeInvalidRequest,
			ErrMsg: err.Error(),
		})
		return
	}

	docID := c.Param("doc_id")
	result, err := service.GetFinanceDocService().ApproveFinanceDoc(c.Request.Context(), docID, req)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}
