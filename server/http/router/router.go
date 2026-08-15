package router

import (
	"context"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	commonauth "github.com/nusiss-capstone-project/identity-mservice/common/auth"
	"github.com/nusiss-capstone-project/reward-mservice/server/config"
	_ "github.com/nusiss-capstone-project/reward-mservice/server/docs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/api"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	swaggerFiles "github.com/swaggo/files"
	gs "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

const (
	serviceURIPrefix = "/reward-ms/v1"
)

func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(log.RecoveryMiddleware())
	r.Use(otelgin.Middleware(data.ServiceName))
	r.Use(log.HTTPResponseIDMiddleware())
	r.Use(corsMiddleware())

	campaignOps := commonauth.RequireRole([]string{
		commonauth.RoleCampaignOps, commonauth.RoleAdmin,
	})
	financeAdmin := commonauth.RequireRole([]string{
		commonauth.RoleFinanceAdmin, commonauth.RoleAdmin,
	})
	campaignOpsOrFinanceAdmin := commonauth.RequireRole([]string{
		commonauth.RoleCampaignOps, commonauth.RoleFinanceAdmin, commonauth.RoleAdmin,
	})

	basicGroup := r.Group(serviceURIPrefix)
	{
		// High-frequency / non-business routes: no HTTP access log.
		basicGroup.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "pong",
			})
		})
		basicGroup.GET("/swagger/*any", gs.WrapHandler(
			swaggerFiles.Handler,
			gs.URL("/reward-ms/v1/swagger/doc.json"),
		))

		// Business routes: enable request access logging.
		apiGroup := basicGroup.Group("")
		apiGroup.Use(commonauth.AuditMiddleware(func(ctx context.Context) commonauth.AuditLogger {
			return log.WithContext(ctx)
		}))
		apiGroup.Use(log.HTTPObservabilityMiddleware())
		{
			apiGroup.POST("/items", api.CreateItem)
			apiGroup.GET("/items/:item_id", api.GetItems)

			adminGroup := apiGroup.Group("/admin")
			{
				// project manage -> campaign_ops; project view -> campaign_ops + finance_admin
				adminGroup.POST("/projects", campaignOps, api.CreateProject)
				adminGroup.GET("/projects", campaignOpsOrFinanceAdmin, api.ListProjects)
				adminGroup.GET("/projects/ongoing", campaignOps, api.ListProjectsWithOngoingIssueRequest)

				// finance_doc edit -> campaign_ops; approval -> finance_admin; view -> both
				adminGroup.POST("/finance-docs", campaignOps, api.CreateFinanceDoc)
				adminGroup.GET("/finance-docs", campaignOpsOrFinanceAdmin, api.ListFinanceDocs)
				adminGroup.GET("/finance-docs/:doc_id", campaignOpsOrFinanceAdmin, api.GetFinanceDocDetail)
				adminGroup.PUT("/finance-docs/:doc_id", campaignOps, api.UpdateFinanceDoc)
				adminGroup.PATCH("/finance-docs/:doc_id/submission", campaignOps, api.SubmitFinanceDocForApproval)
				adminGroup.PATCH("/finance-docs/:doc_id/approval", financeAdmin, api.ApproveFinanceDoc)

				// finance_payment -> finance_admin
				adminGroup.POST("/finance-docs/:doc_id/finance-payments", financeAdmin, api.CreateFinancePayment)
				adminGroup.GET("/finance-docs/:doc_id/finance-payments", financeAdmin, api.GetFinancePaymentListByDocID)
				adminGroup.GET("/finance-docs/:doc_id/finance-payments/:payment_id", financeAdmin, api.GetFinancePayment)

				// issue_request edit/approve -> campaign_ops; view -> both
				adminGroup.POST("/finance-docs/:doc_id/issue-requests", campaignOps, api.CreateIssueRequest)
				adminGroup.GET("/finance-docs/:doc_id/issue-requests", campaignOpsOrFinanceAdmin, api.ListIssueRequestsByDocID)
				adminGroup.PUT("/finance-docs/:doc_id/issue-requests/:issue_request_id", campaignOps, api.UpdateIssueRequest)
				adminGroup.PATCH("/finance-docs/:doc_id/issue-requests/:issue_request_id/submission", campaignOps, api.SubmitIssueRequestForApproval)
				adminGroup.PATCH("/finance-docs/:doc_id/issue-requests/:issue_request_id/approval", financeAdmin, api.ApproveIssueRequest)

				// budgets view -> campaign_ops + finance_admin
				adminGroup.GET("/finance-docs/:doc_id/project-budgets", campaignOpsOrFinanceAdmin, api.ListProjectBudgetsByFinanceDoc)
				adminGroup.GET("/issue-requests/:issue_request_id/issue-budgets", campaignOpsOrFinanceAdmin, api.ListIssueBudgetsByIssueRequest)

				// issue records view -> campaign_ops
				adminGroup.GET("/issue-records/projects/:project_id/users/:user_id", campaignOps, api.ListAdminIssueRecords)

				// payment-configs view -> both
				adminGroup.GET("/payment-configs", campaignOpsOrFinanceAdmin, api.ListPaymentConfigs)

				// templates manage -> campaign_ops
				adminGroup.POST("/templates", campaignOps, api.CreateTemplate)
				adminGroup.GET("/templates", campaignOps, api.ListTemplates)
				adminGroup.PUT("/templates/:template_id", campaignOps, api.UpdateTemplate)
				adminGroup.PUT("/templates/:template_id/publish", campaignOps, api.PublishTemplate)
			}

			webGroup := apiGroup.Group("/web")
			webGroup.Use(commonauth.RequireUser())
			{
				webGroup.GET("/issue-records/projects/:project_id", api.ListWebIssueRecords)
			}
		}
	}
	return r
}

func corsMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: allowedOrigins(),
		AllowMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
		},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Accept", "Authorization",
			commonauth.HeaderInternalUserID, commonauth.HeaderUserRole,
			log.RequestIDHeader, log.TraceIDHeader,
		},
		ExposeHeaders: []string{
			"Content-Length", commonauth.HeaderInternalUserID, commonauth.HeaderUserRole,
			log.RequestIDHeader, log.TraceIDHeader,
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}

func allowedOrigins() []string {
	if config.Config == nil || config.Config.SystemConfig == nil {
		return []string{}
	}
	return config.Config.SystemConfig.AllowedOrigins
}
