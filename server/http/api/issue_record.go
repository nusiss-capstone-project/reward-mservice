package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	commonauth "github.com/nusiss-capstone-project/identity-mservice/common/auth"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

// ListAdminIssueRecords lists issue records for a project user (admin).
//
// @Summary List issue records by project and user
// @Description List all issue records for a user in a project. campaign_ops and admin only.
// @Tags Admin-IssueRecord
// @Produce json
// @Param project_id path int true "Project ID"
// @Param user_id path int true "User ID"
// @Success 200 {object} data.BaseResponse{data=[]data.IssueRecordVO}
// @Failure 400 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/issue-records/projects/{project_id}/users/{user_id} [get]
func ListAdminIssueRecords(c *gin.Context) {
	projectID, err := parsePositiveInt64Param(c, "project_id")
	if err != nil {
		WriteError(c, err)
		return
	}
	userID, err := parsePositiveInt64Param(c, "user_id")
	if err != nil {
		WriteError(c, err)
		return
	}

	result, err := service.GetIssueRecordService().ListIssueRecordsByProjectAndUser(c.Request.Context(), projectID, userID)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// ListWebIssueRecords lists issue records for the current user in a project (web).
//
// @Summary List my issue records by project
// @Description List all issue records for the authenticated user in a project.
// @Tags Web-IssueRecord
// @Produce json
// @Param project_id path int true "Project ID"
// @Success 200 {object} data.BaseResponse{data=[]data.IssueRecordVO}
// @Failure 400 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/web/issue-records/projects/{project_id} [get]
func ListWebIssueRecords(c *gin.Context) {
	projectID, err := parsePositiveInt64Param(c, "project_id")
	if err != nil {
		WriteError(c, err)
		return
	}
	userID, ok := commonauth.GetUserID(c.Request.Context())
	if !ok || userID <= 0 {
		WriteError(c, errs.New(errs.CodeInvalidRequest, "authentication required"))
		return
	}

	result, err := service.GetIssueRecordService().ListIssueRecordsByProjectAndUser(c.Request.Context(), projectID, userID)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

func parsePositiveInt64Param(c *gin.Context, name string) (int64, error) {
	raw := c.Param(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errs.New(errs.CodeInvalidRequest, "invalid "+name)
	}
	return id, nil
}
