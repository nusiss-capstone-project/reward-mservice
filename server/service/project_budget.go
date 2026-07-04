package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
)

type ProjectBudgetService interface {
	InitFromApprovedDoc(ctx context.Context, docID string) error
}

type ProjectBudgetServiceImpl struct {
	financeDocDao    dao.FinanceDocDao
	paymentConfigDao dao.PaymentConfigDao
	projectBudgetDao dao.ProjectBudgetDao
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
			txBeginner:       repository.DB,
		}
	})
	return projectBudgetServiceInst
}

func (s *ProjectBudgetServiceImpl) InitFromApprovedDoc(ctx context.Context, docID string) error {
	logger := log.WithContext(ctx)
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return errs.New(errs.CodeInvalidRequest, "doc_id is required")
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
