package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
)

type budgetItem struct {
	VoucherType string
	Unit        string
	Amount      string
}

func resolveBudgetItems(
	ctx context.Context,
	paymentConfigDao dao.PaymentConfigDao,
	detail []model.ApplicationDetailItem,
) ([]budgetItem, error) {
	if len(detail) == 0 {
		return nil, errs.New(errs.CodeInvalidRequest, "application_detail is empty")
	}

	items := make([]budgetItem, 0, len(detail))
	seen := make(map[string]struct{}, len(detail))
	for _, item := range detail {
		if strings.TrimSpace(item.PayAddress) == "" ||
			strings.TrimSpace(item.Amount) == "" {
			return nil, errs.New(errs.CodeInvalidRequest, "application_detail fields are required")
		}

		payAddress := strings.TrimSpace(item.PayAddress)
		amount := strings.TrimSpace(item.Amount)
		cfg, err := paymentConfigDao.GetByPayAddress(ctx, payAddress)
		if err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, err)
		}
		if cfg == nil {
			return nil, errs.New(errs.CodeInvalidPayAddress, "pay address not found: "+payAddress)
		}

		key := fmt.Sprintf("%s:%s", cfg.VoucherType, cfg.Unit)
		if _, ok := seen[key]; ok {
			return nil, errs.New(errs.CodeDuplicateBudgetPair, "")
		}
		seen[key] = struct{}{}

		items = append(items, budgetItem{
			VoucherType: cfg.VoucherType,
			Unit:        cfg.Unit,
			Amount:      amount,
		})
	}
	return items, nil
}

func toApplicationDetailItems(detail []data.ApplicationDetailItemVO) []model.ApplicationDetailItem {
	items := make([]model.ApplicationDetailItem, 0, len(detail))
	for _, item := range detail {
		items = append(items, model.ApplicationDetailItem{
			PayAddress: strings.TrimSpace(item.PayAddress),
			Amount:     strings.TrimSpace(item.Amount),
		})
	}
	return items
}

func toProjectBudgets(docID string, projectID int64, items []budgetItem) []*model.ProjectBudget {
	result := make([]*model.ProjectBudget, 0, len(items))
	for _, item := range items {
		result = append(result, &model.ProjectBudget{
			FinanceDocID:    docID,
			ProjectID:       projectID,
			VoucherType:     item.VoucherType,
			Unit:            item.Unit,
			TotalAmount:     "0",
			AvailableAmount: item.Amount,
			WithholdAmount:  "0",
			IssuedAmount:    "0",
			RefundAmount:    "0",
		})
	}
	return result
}
