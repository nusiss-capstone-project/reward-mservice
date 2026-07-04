package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
)

func HTTPStatusFromCode(code int) int {
	switch code {
	case errs.CodeInvalidRequest, errs.CodeInvalidStatusTransition, errs.CodeInvalidPayAddress, errs.CodeInvalidPagination,
		errs.CodeFinanceDocProjectExists, errs.CodeDuplicateBudgetPair, errs.CodePaymentExceedsDocAmount,
		errs.CodeFinanceDocNotApproved:
		return http.StatusBadRequest
	case errs.CodeProjectNotFound, errs.CodeFinanceDocNotFound, errs.CodeProjectBudgetNotFound,
		errs.CodeFinancePaymentNotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func WriteSuccess(c *gin.Context, payload interface{}) {
	c.JSON(http.StatusOK, data.BaseResponse{
		Code: errs.CodeOK,
		Data: payload,
	})
}

func WriteError(c *gin.Context, err error) {
	fields := []any{"route", c.FullPath(), "method", c.Request.Method}
	if docID := c.Param("doc_id"); docID != "" {
		fields = append(fields, "doc_id", docID)
	}
	if paymentID := c.Param("payment_id"); paymentID != "" {
		fields = append(fields, "payment_id", paymentID)
	}
	log.LogAppError(c.Request.Context(), err, "http request failed", fields...)

	var appErr *errs.AppError
	if errors.As(err, &appErr) {
		c.JSON(HTTPStatusFromCode(appErr.Code), data.BaseResponse{
			Code:   appErr.Code,
			ErrMsg: appErr.Error(),
		})
		return
	}
	c.JSON(http.StatusInternalServerError, data.BaseResponse{
		Code:   errs.CodeInternalError,
		ErrMsg: errs.Message(errs.CodeInternalError),
	})
}
