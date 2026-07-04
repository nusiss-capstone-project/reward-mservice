package dao

import (
	"context"
	"errors"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProjectBudgetDao interface {
	BatchCreate(ctx context.Context, tx *gorm.DB, budgets []*model.ProjectBudget) error
	CountByFinanceDocID(ctx context.Context, docID string) (int64, error)
	GetByDocIDVoucherTypeUnit(ctx context.Context, docID, voucherType, unit string) (*model.ProjectBudget, error)
	GetByProjectIDVoucherTypeUnit(ctx context.Context, projectID int64, voucherType, unit string) (*model.ProjectBudget, error)
	LockByID(ctx context.Context, tx *gorm.DB, budgetID int64) (*model.ProjectBudget, error)
	UpdateTotalAndAvailableAmount(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error
	ApplySubmitWithhold(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error
	ApplyApproveIssued(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error
	ApplyRejectRelease(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error
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

func (d *ProjectBudgetDaoImpl) BatchCreate(ctx context.Context, tx *gorm.DB, budgets []*model.ProjectBudget) error {
	if len(budgets) == 0 {
		return nil
	}
	err := dbFrom(d.db, tx).WithContext(ctx).Create(budgets).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrBudgetAlreadyExists
		}
		log.WithContext(ctx).Errorf("batch create project budget failed: count=%d err=%v", len(budgets), err)
		return err
	}
	return nil
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

func (d *ProjectBudgetDaoImpl) GetByDocIDVoucherTypeUnit(
	ctx context.Context,
	docID, voucherType, unit string,
) (*model.ProjectBudget, error) {
	var budget model.ProjectBudget
	err := d.db.WithContext(ctx).
		Where("finance_doc_id = ? AND voucher_type = ? AND unit = ?", docID, voucherType, unit).
		First(&budget).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorf("get project budget failed: doc_id=%s voucher_type=%s unit=%s err=%v",
			docID, voucherType, unit, err)
		return nil, err
	}
	return &budget, nil
}

func (d *ProjectBudgetDaoImpl) GetByProjectIDVoucherTypeUnit(
	ctx context.Context,
	projectID int64,
	voucherType, unit string,
) (*model.ProjectBudget, error) {
	var budget model.ProjectBudget
	err := d.db.WithContext(ctx).
		Where("project_id = ? AND voucher_type = ? AND unit = ?", projectID, voucherType, unit).
		First(&budget).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorf("get project budget by project failed: project_id=%d voucher_type=%s unit=%s err=%v",
			projectID, voucherType, unit, err)
		return nil, err
	}
	return &budget, nil
}

func (d *ProjectBudgetDaoImpl) LockByID(ctx context.Context, tx *gorm.DB, budgetID int64) (*model.ProjectBudget, error) {
	var budget model.ProjectBudget
	err := dbFrom(d.db, tx).WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", budgetID).
		First(&budget).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorf("lock project budget failed: budget_id=%d err=%v", budgetID, err)
		return nil, err
	}
	log.WithContext(ctx).Infof("project budget locked: budget_id=%d", budgetID)
	return &budget, nil
}

func (d *ProjectBudgetDaoImpl) UpdateTotalAndAvailableAmount(
	ctx context.Context,
	tx *gorm.DB,
	budgetID int64,
	amount string,
) error {
	err := dbFrom(d.db, tx).WithContext(ctx).
		Model(&model.ProjectBudget{}).
		Where("id = ?", budgetID).
		Updates(map[string]interface{}{
			"total_amount":     gorm.Expr("total_amount + ?", amount),
			"available_amount": gorm.Expr("available_amount + ?", amount),
		}).Error
	if err != nil {
		log.WithContext(ctx).Errorf("update project budget amounts failed: budget_id=%d amount=%s err=%v", budgetID, amount, err)
		return err
	}
	log.WithContext(ctx).Infof("project budget amounts updated: budget_id=%d amount=%s", budgetID, amount)
	return nil
}

func (d *ProjectBudgetDaoImpl) ApplySubmitWithhold(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error {
	result := dbFrom(d.db, tx).WithContext(ctx).Model(&model.ProjectBudget{}).
		Where("id = ?", budgetID).
		Updates(map[string]interface{}{
			"available_amount": gorm.Expr("available_amount - ?", amount),
			"withhold_amount":   gorm.Expr("withhold_amount + ?", amount),
		})
	if result.Error != nil {
		log.WithContext(ctx).Errorf("apply submit withhold failed: budget_id=%d amount=%s err=%v", budgetID, amount, result.Error)
		return result.Error
	}
	return nil
}

func (d *ProjectBudgetDaoImpl) ApplyApproveIssued(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error {
	result := dbFrom(d.db, tx).WithContext(ctx).Model(&model.ProjectBudget{}).
		Where("id = ?", budgetID).
		Updates(map[string]interface{}{
			"issued_amount":  gorm.Expr("issued_amount + ?", amount),
			"withhold_amount": gorm.Expr("withhold_amount - ?", amount),
		})
	if result.Error != nil {
		log.WithContext(ctx).Errorf("apply approve issued failed: budget_id=%d amount=%s err=%v", budgetID, amount, result.Error)
		return result.Error
	}
	return nil
}

func (d *ProjectBudgetDaoImpl) ApplyRejectRelease(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error {
	result := dbFrom(d.db, tx).WithContext(ctx).Model(&model.ProjectBudget{}).
		Where("id = ?", budgetID).
		Updates(map[string]interface{}{
			"available_amount": gorm.Expr("available_amount + ?", amount),
			"withhold_amount":   gorm.Expr("withhold_amount - ?", amount),
		})
	if result.Error != nil {
		log.WithContext(ctx).Errorf("apply reject release failed: budget_id=%d amount=%s err=%v", budgetID, amount, result.Error)
		return result.Error
	}
	return nil
}

var ErrBudgetAlreadyExists = errors.New("project budget already exists")
