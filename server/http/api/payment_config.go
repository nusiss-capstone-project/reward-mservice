package api

import (
	"github.com/gin-gonic/gin"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

// ListPaymentConfigs lists payment config options.
//
// @Summary List payment configs
// @Description List payment address and account options for finance doc creation.
// @Tags Admin-PaymentConfig
// @Produce json
// @Success 200 {object} data.BaseResponse{data=[]data.PaymentConfigVO}
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/payment-configs [get]
func ListPaymentConfigs(c *gin.Context) {
	result, err := service.GetPaymentConfigService().ListPaymentConfigs(c.Request.Context())
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}
