package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

// CreateProject creates a project.
//
// @Summary Create project
// @Description Create a reward project.
// @Tags Admin-Project
// @Accept json
// @Produce json
// @Param body body data.CreateProjectRequest true "Project payload"
// @Success 200 {object} data.BaseResponse{data=data.CreateProjectResponse}
// @Failure 400 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/projects [post]
func CreateProject(c *gin.Context) {
	req := &data.CreateProjectRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{
			Code:   errs.CodeInvalidRequest,
			ErrMsg: err.Error(),
		})
		return
	}

	projectID, err := service.GetProjectService().CreateProject(c.Request.Context(), req)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, &data.CreateProjectResponse{ProjectID: projectID})
}

// ListProjects lists projects with pagination.
//
// @Summary List projects
// @Description List reward projects with page and size.
// @Tags Admin-Project
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(20)
// @Success 200 {object} data.BaseResponse{data=data.PageResult}
// @Failure 400 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/projects [get]
func ListProjects(c *gin.Context) {
	query := data.PageQuery{}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{
			Code:   errs.CodeInvalidRequest,
			ErrMsg: err.Error(),
		})
		return
	}
	page, size := query.Normalize()

	result, err := service.GetProjectService().ListProjects(c.Request.Context(), page, size)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}
