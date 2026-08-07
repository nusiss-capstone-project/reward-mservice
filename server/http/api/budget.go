package api

import (
	"github.com/gin-gonic/gin"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

// ListProjectBudgetsByFinanceDoc lists project budgets under a finance doc.
//
// @Summary List project budgets by finance doc
// @Description List project budgets for a finance doc. campaign_ops, finance_admin and admin.
// @Tags Admin-Budget
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Success 200 {object} data.BaseResponse{data=[]data.BudgetVO}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id}/project-budgets [get]
func ListProjectBudgetsByFinanceDoc(c *gin.Context) {
	result, err := service.GetProjectBudgetService().ListByFinanceDocID(c.Request.Context(), c.Param("doc_id"))
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// ListIssueBudgetsByIssueRequest lists issue budgets under an issue request.
//
// @Summary List issue budgets by issue request
// @Description List issue budgets for an issue request. campaign_ops, finance_admin and admin.
// @Tags Admin-Budget
// @Produce json
// @Param issue_request_id path int true "Issue request ID"
// @Success 200 {object} data.BaseResponse{data=[]data.BudgetVO}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/issue-requests/{issue_request_id}/issue-budgets [get]
func ListIssueBudgetsByIssueRequest(c *gin.Context) {
	issueRequestID, err := parseIssueRequestIDParam(c)
	if err != nil {
		WriteError(c, err)
		return
	}
	result, err := service.GetProjectBudgetService().ListIssueBudgetByIssueRequestID(c.Request.Context(), issueRequestID)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}
