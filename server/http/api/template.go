package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

// CreateTemplate creates a template in DRAFT status.
//
// @Summary Create template
// @Description Create a reward template for campaign ops.
// @Tags Admin-Template
// @Accept json
// @Produce json
// @Param body body data.CreateTemplateRequest true "Template payload"
// @Success 200 {object} data.BaseResponse{data=data.CreateTemplateResponse}
// @Failure 400 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/templates [post]
func CreateTemplate(c *gin.Context) {
	req := &data.CreateTemplateRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{
			Code:   errs.CodeInvalidRequest,
			ErrMsg: err.Error(),
		})
		return
	}

	templateID, err := service.GetTemplateService().CreateTemplate(c.Request.Context(), req)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, &data.CreateTemplateResponse{TemplateID: templateID})
}

// UpdateTemplate updates a draft template.
//
// @Summary Update template
// @Description Update template config in DRAFT status only.
// @Tags Admin-Template
// @Accept json
// @Produce json
// @Param template_id path int true "Template ID"
// @Param body body data.UpdateTemplateRequest true "Template payload"
// @Success 200 {object} data.BaseResponse{data=data.TemplateVO}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/templates/{template_id} [put]
func UpdateTemplate(c *gin.Context) {
	req := &data.UpdateTemplateRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{
			Code:   errs.CodeInvalidRequest,
			ErrMsg: err.Error(),
		})
		return
	}

	templateID, err := parseTemplateIDParam(c)
	if err != nil {
		WriteError(c, err)
		return
	}

	result, err := service.GetTemplateService().UpdateTemplate(c.Request.Context(), templateID, req)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// ListTemplates lists templates with pagination.
//
// @Summary List templates
// @Description List reward templates for campaign ops.
// @Tags Admin-Template
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(20)
// @Success 200 {object} data.BaseResponse{data=data.PageResult}
// @Failure 400 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/templates [get]
func ListTemplates(c *gin.Context) {
	query := data.PageQuery{}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, data.BaseResponse{
			Code:   errs.CodeInvalidRequest,
			ErrMsg: err.Error(),
		})
		return
	}
	page, size := query.Normalize()

	result, err := service.GetTemplateService().ListTemplates(c.Request.Context(), page, size)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

// PublishTemplate publishes a draft template.
//
// @Summary Publish template
// @Description Move template from DRAFT to PUBLISHED.
// @Tags Admin-Template
// @Produce json
// @Param template_id path int true "Template ID"
// @Success 200 {object} data.BaseResponse{data=data.PublishTemplateResponse}
// @Failure 400 {object} data.BaseResponse
// @Failure 404 {object} data.BaseResponse
// @Failure 500 {object} data.BaseResponse
// @Router /reward-ms/v1/admin/templates/{template_id}/publish [put]
func PublishTemplate(c *gin.Context) {
	templateID, err := parseTemplateIDParam(c)
	if err != nil {
		WriteError(c, err)
		return
	}

	result, err := service.GetTemplateService().PublishTemplate(c.Request.Context(), templateID)
	if err != nil {
		WriteError(c, err)
		return
	}
	WriteSuccess(c, result)
}

func parseTemplateIDParam(c *gin.Context) (int64, error) {
	return service.ParseTemplateID(c.Param("template_id"))
}
