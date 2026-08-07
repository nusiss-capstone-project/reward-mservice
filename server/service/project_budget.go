package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
)

type ProjectBudgetService interface {
	InitFromApprovedDoc(ctx context.Context, docID string) error
	ListByFinanceDocID(ctx context.Context, docID string) ([]*data.BudgetVO, error)
	ListIssueBudgetByIssueRequestID(ctx context.Context, issueRequestID int64) ([]*data.BudgetVO, error)
}

type ProjectBudgetServiceImpl struct {
	financeDocDao    dao.FinanceDocDao
	paymentConfigDao dao.PaymentConfigDao
	projectBudgetDao dao.ProjectBudgetDao
	issueBudgetDao   dao.IssueBudgetDao
	issueRequestDao  dao.IssueRequestDao
	txBeginner       repository.TxBeginner
}

var (
	projectBudgetServiceOnce sync.Once
	projectBudgetServiceInst ProjectBudgetService
)

func GetProjectBudgetService() ProjectBudgetService {
	projectBudgetServiceOnce.Do(func() {
		projectBudgetServiceInst = &ProjectBudgetServiceImpl{
			financeDocDao:    dao.GetFinanceDocDao(),
			paymentConfigDao: dao.GetPaymentConfigDao(),
			projectBudgetDao: dao.GetProjectBudgetDao(),
			issueBudgetDao:   dao.GetIssueBudgetDao(),
			issueRequestDao:  dao.GetIssueRequestDao(),
			txBeginner:       repository.DB,
		}
	})
	return projectBudgetServiceInst
}

func (s *ProjectBudgetServiceImpl) InitFromApprovedDoc(ctx context.Context, docID string) error {
	logger := log.WithContext(ctx)
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return errs.New(errs.CodeInvalidRequest, errs.MsgDocIDRequired)
	}

	doc, err := s.financeDocDao.GetByDocID(ctx, docID)
	if err != nil {
		logger.Errorf("load finance doc failed: doc_id=%s err=%v", docID, err)
		return errs.Wrap(errs.CodeInternalError, err)
	}
	if doc == nil {
		return errs.New(errs.CodeFinanceDocNotFound, "")
	}
	if doc.Status != model.FinanceDocStatusApproved {
		return errs.New(errs.CodeInvalidStatusTransition, "finance doc is not approved")
	}

	detail := make([]model.ApplicationDetailItem, 0)
	if len(doc.ApplicationDetail) > 0 {
		if err := json.Unmarshal(doc.ApplicationDetail, &detail); err != nil {
			logger.Errorf("unmarshal application detail failed: doc_id=%s err=%v", docID, err)
			return errs.Wrap(errs.CodeInternalError, err)
		}
	}

	items, err := resolveBudgetItems(ctx, s.paymentConfigDao, detail)
	if err != nil {
		return err
	}

	budgets := toProjectBudgets(docID, doc.ProjectID, items)
	err = s.txBeginner.Transaction(func(tx *gorm.DB) error {
		return s.projectBudgetDao.BatchCreate(ctx, tx, budgets)
	})
	if err != nil {
		if errors.Is(err, dao.ErrBudgetAlreadyExists) {
			logger.Infof("project budget already initialized: doc_id=%s", docID)
			return nil

		}
		logger.Errorf("create project budget failed: doc_id=%s err=%v", docID, err)
		return errs.Wrap(errs.CodeInternalError, err)
	}

	logger.Infof("project budget created: doc_id=%s project_id=%d items=%d", docID, doc.ProjectID, len(items))
	return nil
}

func (s *ProjectBudgetServiceImpl) ListByFinanceDocID(ctx context.Context, docID string) ([]*data.BudgetVO, error) {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return nil, errs.New(errs.CodeInvalidRequest, errs.MsgDocIDRequired)
	}

	doc, err := s.financeDocDao.GetByDocID(ctx, docID)
	if err != nil {
		log.WithContext(ctx).Errorw("load finance doc for project budgets failed", "doc_id", docID, "error", err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	if doc == nil {
		return nil, errs.New(errs.CodeFinanceDocNotFound, "")
	}

	budgets, err := s.projectBudgetDao.ListByFinanceDocID(ctx, docID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	items := make([]*data.BudgetVO, 0, len(budgets))
	for _, budget := range budgets {
		items = append(items, toBudgetVOFromProject(budget))
	}
	return items, nil
}

func (s *ProjectBudgetServiceImpl) ListIssueBudgetByIssueRequestID(
	ctx context.Context,
	issueRequestID int64,
) ([]*data.BudgetVO, error) {
	if issueRequestID <= 0 {
		return nil, errs.New(errs.CodeInvalidRequest, "issue_request_id must be positive")
	}

	request, err := s.issueRequestDao.GetByID(ctx, issueRequestID)
	if err != nil {
		log.WithContext(ctx).Errorw("load issue request for issue budget failed",
			"issue_request_id", issueRequestID, "error", err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	if request == nil {
		return nil, errs.New(errs.CodeIssueRequestNotFound, "")
	}

	budget, err := s.issueBudgetDao.GetByIssueRequestID(ctx, issueRequestID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	if budget == nil {
		return []*data.BudgetVO{}, nil
	}
	return []*data.BudgetVO{toBudgetVOFromIssue(budget)}, nil
}

func toBudgetVOFromProject(budget *model.ProjectBudget) *data.BudgetVO {
	return &data.BudgetVO{
		VoucherType:     budget.VoucherType,
		Unit:            budget.Unit,
		AvailableAmount: budget.AvailableAmount,
		TotalAmount:     budget.TotalAmount,
		IssuedAmount:    budget.IssuedAmount,
	}
}

func toBudgetVOFromIssue(budget *model.IssueBudget) *data.BudgetVO {
	return &data.BudgetVO{
		VoucherType:     budget.VoucherType,
		Unit:            budget.Unit,
		AvailableAmount: budget.AvailableAmount,
		TotalAmount:     budget.TotalAmount,
		IssuedAmount:    budget.IssuedAmount,
	}
}
