package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

// CreateFinancePayment creates a finance payment under a finance doc.
//
// @Summary Create finance payment
// @Description Create a finance payment and update project budget total_amount.
// @Tags Admin-FinancePayment
// @Accept json
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Param body body data.CreateFinancePaymentRequest true "Finance payment payload"
// @Success 200 {object} data.BaseResponse{data=data.FinancePaymentVO}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id}/finance-payments [post]
func CreateFinancePayment(c *gin.Context) {
	req := &data.CreateFinancePaymentRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		appErr := errs.New(errs.CodeInvalidRequest, err.Error())
		log.LogAppError(c.Request.Context(), appErr, "create finance payment bind failed", "doc_id", c.Param("doc_id"))
		c.JSON(http.StatusBadRequest, data.BaseResponse{
			Code:   errs.CodeInvalidRequest,
			ErrMsg: err.Error(),
		})
		return
	}

	docID := c.Param("doc_id")
	result, err := service.GetFinancePaymentService().CreateFinancePayment(c.Request.Context(), docID, req)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// GetFinancePayment returns a finance payment by doc_id and payment_id.
//
// @Summary Get finance payment
// @Description Get finance payment detail under a finance doc.
// @Tags Admin-FinancePayment
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Param payment_id path string true "Finance payment ID"
// @Success 200 {object} data.BaseResponse{data=data.FinancePaymentVO}
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id}/finance-payments/{payment_id} [get]
func GetFinancePayment(c *gin.Context) {
	docID := c.Param("doc_id")
	paymentID := c.Param("payment_id")
	result, err := service.GetFinancePaymentService().GetFinancePayment(c.Request.Context(), docID, paymentID)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// GetFinancePaymentListByDocID returns a list of finance payments by doc_id.
//
// @Summary Get finance payment list
// @Description Get a list of finance payments under a finance doc.
// @Tags Admin-FinancePayment
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Success 200 {object} data.BaseResponse{data=[]data.FinancePaymentVO}
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id}/finance-payments [get]
func GetFinancePaymentListByDocID(c *gin.Context) {
	docID := c.Param("doc_id")
	result, err := service.GetFinancePaymentService().GetFinancePaymentListByDocID(c.Request.Context(), docID)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}
