package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

// CreateIssueRequest creates an issue request under a finance doc.
//
// @Summary Create issue request
// @Description Create an issue request for campaign budget allocation.
// @Tags Admin-IssueRequest
// @Accept json
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Param body body data.CreateIssueRequestRequest true "Issue request payload"
// @Success 200 {object} data.BaseResponse{data=data.IssueRequestVO}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id}/issue-requests [post]
func CreateIssueRequest(c *gin.Context) {
	req := &data.CreateIssueRequestRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		appErr := errs.New(errs.CodeInvalidRequest, err.Error())
		log.LogAppError(c.Request.Context(), appErr, "create issue request bind failed", "doc_id", c.Param("doc_id"))
		c.JSON(http.StatusBadRequest, data.BaseResponse{Code: errs.CodeInvalidRequest, ErrMsg: err.Error()})
		return
	}

	result, err := service.GetIssueRequestService().CreateIssueRequest(c.Request.Context(), c.Param("doc_id"), req)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// UpdateIssueRequest updates an editable issue request.
//
// @Summary Update issue request
// @Description Update issue request amount, voucher_type and unit in DRAFT or REJECTED status.
// @Tags Admin-IssueRequest
// @Accept json
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Param issue_request_id path int true "Issue request ID"
// @Param body body data.UpdateIssueRequestRequest true "Issue request payload"
// @Success 200 {object} data.BaseResponse{data=data.IssueRequestVO}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id}/issue-requests/{issue_request_id} [put]
func UpdateIssueRequest(c *gin.Context) {
	req := &data.UpdateIssueRequestRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		appErr := errs.New(errs.CodeInvalidRequest, err.Error())
		log.LogAppError(c.Request.Context(), appErr, "update issue request bind failed", "doc_id", c.Param("doc_id"))
		c.JSON(http.StatusBadRequest, data.BaseResponse{Code: errs.CodeInvalidRequest, ErrMsg: err.Error()})
		return
	}

	issueRequestID, err := parseIssueRequestIDParam(c)
	if err != nil {
		WriteError(c, err)
		return
	}

	result, err := service.GetIssueRequestService().UpdateIssueRequest(c.Request.Context(), c.Param("doc_id"), issueRequestID, req)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// ListIssueRequestsByDocID lists issue requests under a finance doc.
//
// @Summary List issue requests
// @Description List issue requests for a finance doc.
// @Tags Admin-IssueRequest
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(20)
// @Success 200 {object} data.BaseResponse{data=data.PageResult}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id}/issue-requests [get]
func ListIssueRequestsByDocID(c *gin.Context) {
	query := data.PageQuery{}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{Code: errs.CodeInvalidRequest, ErrMsg: err.Error()})
		return
	}
	page, size := query.Normalize()

	result, err := service.GetIssueRequestService().ListIssueRequestsByDocID(c.Request.Context(), c.Param("doc_id"), page, size)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// SubmitIssueRequestForApproval submits an issue request for approval.
//
// @Summary Submit issue request
// @Description Submit issue request and publish budget withhold event.
// @Tags Admin-IssueRequest
// @Accept json
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Param issue_request_id path int true "Issue request ID"
// @Param body body data.SubmitIssueRequestRequest true "Submit payload"
// @Success 200 {object} data.BaseResponse{data=data.UpdateIssueRequestResponse}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id}/issue-requests/{issue_request_id}/submission [patch]
func SubmitIssueRequestForApproval(c *gin.Context) {
	req := &data.SubmitIssueRequestRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{Code: errs.CodeInvalidRequest, ErrMsg: err.Error()})
		return
	}

	issueRequestID, err := parseIssueRequestIDParam(c)
	if err != nil {
		WriteError(c, err)
		return
	}

	result, err := service.GetIssueRequestService().SubmitIssueRequest(c.Request.Context(), c.Param("doc_id"), issueRequestID, req)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// ApproveIssueRequest approves or rejects an issue request.
//
// @Summary Approve or reject issue request
// @Description Approve or reject issue request and publish budget update event.
// @Tags Admin-IssueRequest
// @Accept json
// @Produce json
// @Param doc_id path string true "Finance doc ID"
// @Param issue_request_id path int true "Issue request ID"
// @Param body body data.ApproveIssueRequestRequest true "Approval payload"
// @Success 200 {object} data.BaseResponse{data=data.UpdateIssueRequestResponse}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/finance-docs/{doc_id}/issue-requests/{issue_request_id}/approval [patch]
func ApproveIssueRequest(c *gin.Context) {
	req := &data.ApproveIssueRequestRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{Code: errs.CodeInvalidRequest, ErrMsg: err.Error()})
		return
	}

	issueRequestID, err := parseIssueRequestIDParam(c)
	if err != nil {
		WriteError(c, err)
		return
	}

	result, err := service.GetIssueRequestService().ApproveIssueRequest(c.Request.Context(), c.Param("doc_id"), issueRequestID, req)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

func parseIssueRequestIDParam(c *gin.Context) (int64, error) {
	return service.ParseIssueRequestID(c.Param("issue_request_id"))
}
