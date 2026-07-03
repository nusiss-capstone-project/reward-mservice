package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
)

func HTTPStatusFromCode(code int) int {
	switch code {
	case errs.CodeInvalidRequest, errs.CodeInvalidStatusTransition, errs.CodeInvalidPayAddress, errs.CodeInvalidPagination:
		return http.StatusBadRequest
	case errs.CodeProjectNotFound, errs.CodeFinanceDocNotFound:
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
