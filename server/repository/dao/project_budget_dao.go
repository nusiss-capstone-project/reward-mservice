package dao

import (
	"context"
	"errors"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
)

type BudgetCreateItem struct {
	VoucherType string
	Unit        string
	Amount      string
}

type ProjectBudgetDao interface {
	CreateFromApprovedDoc(ctx context.Context, docID string, projectID int64, items []BudgetCreateItem) error
	CountByFinanceDocID(ctx context.Context, docID string) (int64, error)
}

type ProjectBudgetDaoImpl struct {
	db *gorm.DB
}

var (
	projectBudgetOnce sync.Once
	projectBudgetDao  ProjectBudgetDao
)

func GetProjectBudgetDao() ProjectBudgetDao {
	projectBudgetOnce.Do(func() {
		projectBudgetDao = &ProjectBudgetDaoImpl{db: repository.DB}
	})
	return projectBudgetDao
}

func (d *ProjectBudgetDaoImpl) CreateFromApprovedDoc(
	ctx context.Context,
	docID string,
	projectID int64,
	items []BudgetCreateItem,
) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var doc model.FinanceDoc
		if err := tx.Where("doc_id = ?", docID).First(&doc).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			log.WithContext(ctx).Errorf("load finance doc for budget create failed: doc_id=%s err=%v", docID, err)
			return err
		}
		if doc.Status != model.FinanceDocStatusApproved {
			return ErrFinanceDocNotApproved
		}

		for _, item := range items {
			budget := &model.ProjectBudget{
				FinanceDocID:    docID,
				ProjectID:       projectID,
				VoucherType:     item.VoucherType,
				Unit:            item.Unit,
				TotalAmount:     item.Amount,
				AvailableAmount: item.Amount,
				WitholdAmount:   "0",
				IssuedAmount:    "0",
				RefundAmount:    "0",
			}
			if err := tx.Create(budget).Error; err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					return ErrBudgetAlreadyExists
				}
				log.WithContext(ctx).Errorf("create project budget failed: doc_id=%s err=%v", docID, err)
				return err
			}
		}
		return nil
	})
}

func (d *ProjectBudgetDaoImpl) CountByFinanceDocID(ctx context.Context, docID string) (int64, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&model.ProjectBudget{}).
		Where("finance_doc_id = ?", docID).
		Count(&count).Error
	if err != nil {
		log.WithContext(ctx).Errorf("count project budget failed: doc_id=%s err=%v", docID, err)
		return 0, err
	}
	return count, nil
}

var (
	ErrFinanceDocNotApproved = errors.New("finance doc is not approved")
	ErrBudgetAlreadyExists   = errors.New("project budget already exists")
)
