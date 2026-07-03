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

	payAddresses := make([]string, 0, len(detail))
	for _, item := range detail {
		if strings.TrimSpace(item.PayAddress) == "" ||
			strings.TrimSpace(item.Amount) == "" ||
			strings.TrimSpace(item.Unit) == "" {
			return nil, errs.New(errs.CodeInvalidRequest, "application_detail fields are required")
		}
		payAddresses = append(payAddresses, strings.TrimSpace(item.PayAddress))
	}

	configMap, err := paymentConfigDao.MapByPayAddresses(ctx, payAddresses)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}

	items := make([]budgetItem, 0, len(detail))
	seen := make(map[string]struct{}, len(detail))
	for _, item := range detail {
		payAddress := strings.TrimSpace(item.PayAddress)
		unit := strings.TrimSpace(item.Unit)
		amount := strings.TrimSpace(item.Amount)
		cfg, ok := configMap[payAddress]
		if !ok {
			return nil, errs.New(errs.CodeInvalidPayAddress, "pay address not found: "+payAddress)
		}

		key := fmt.Sprintf("%s:%s", cfg.VoucherType, unit)
		if _, ok := seen[key]; ok {
			return nil, errs.New(errs.CodeDuplicateBudgetPair, "")
		}
		seen[key] = struct{}{}

		items = append(items, budgetItem{
			VoucherType: cfg.VoucherType,
			Unit:        unit,
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
			Unit:       strings.TrimSpace(item.Unit),
		})
	}
	return items
}

func toBudgetCreateItems(items []budgetItem) []dao.BudgetCreateItem {
	result := make([]dao.BudgetCreateItem, 0, len(items))
	for _, item := range items {
		result = append(result, dao.BudgetCreateItem{
			VoucherType: item.VoucherType,
			Unit:        item.Unit,
			Amount:      item.Amount,
		})
	}
	return result
}
