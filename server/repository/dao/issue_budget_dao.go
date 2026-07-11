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

type IssueBudgetDao interface {
	Create(ctx context.Context, tx *gorm.DB, budget *model.IssueBudget) error
	GetByIssueRequestID(ctx context.Context, issueRequestID int64) (*model.IssueBudget, error)
	GetByID(ctx context.Context, id int64) (*model.IssueBudget, error)
	GetFirstAvailableForDistribution(
		ctx context.Context,
		projectID int64,
		voucherType, unit, amount string,
	) (*model.IssueBudget, error)
	LockByID(ctx context.Context, tx *gorm.DB, budgetID int64) (*model.IssueBudget, error)
	ApplyDistributionDeductAvailable(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error
	ApplyDistributionIssued(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error
	ApplyDistributionRefund(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error
}

type IssueBudgetDaoImpl struct {
	db *gorm.DB
}

var (
	issueBudgetOnce sync.Once
	issueBudgetDao  IssueBudgetDao
)

func GetIssueBudgetDao() IssueBudgetDao {
	issueBudgetOnce.Do(func() {
		issueBudgetDao = &IssueBudgetDaoImpl{db: repository.DB}
	})
	return issueBudgetDao
}

func (d *IssueBudgetDaoImpl) Create(ctx context.Context, tx *gorm.DB, budget *model.IssueBudget) error {
	err := dbFrom(d.db, tx).WithContext(ctx).Create(budget).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrIssueBudgetAlreadyExists
		}
		log.WithContext(ctx).Errorw("create issue budget failed",
			"issue_request_id", budget.IssueRequestID,
			"error", err,
		)
		return err
	}
	return nil
}

func (d *IssueBudgetDaoImpl) GetByIssueRequestID(ctx context.Context, issueRequestID int64) (*model.IssueBudget, error) {
	var budget model.IssueBudget
	err := d.db.WithContext(ctx).Where("issue_request_id = ?", issueRequestID).First(&budget).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorw("get issue budget failed", "issue_request_id", issueRequestID, "error", err)
		return nil, err
	}
	return &budget, nil
}

func (d *IssueBudgetDaoImpl) GetByID(ctx context.Context, id int64) (*model.IssueBudget, error) {
	var budget model.IssueBudget
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&budget).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorw("get issue budget by id failed", "budget_id", id, "error", err)
		return nil, err
	}
	return &budget, nil
}

func (d *IssueBudgetDaoImpl) GetFirstAvailableForDistribution(
	ctx context.Context,
	projectID int64,
	voucherType, unit, amount string,
) (*model.IssueBudget, error) {
	var budget model.IssueBudget
	err := d.db.WithContext(ctx).
		Table("issue_budget AS ib").
		Select("ib.*").
		Joins("JOIN issue_requests ir ON ib.issue_request_id = ir.id").
		Where("ir.project_id = ? AND ib.voucher_type = ? AND ib.unit = ? AND ib.status = ?",
			projectID, voucherType, unit, model.IssueBudgetStatusOngoing).
		Where("ib.available_amount >= ?", amount).
		Order("ib.id ASC").
		Limit(1).
		First(&budget).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorw("find available issue budget failed",
			"project_id", projectID,
			"voucher_type", voucherType,
			"unit", unit,
			"error", err,
		)
		return nil, err
	}
	return &budget, nil
}

func (d *IssueBudgetDaoImpl) LockByID(ctx context.Context, tx *gorm.DB, budgetID int64) (*model.IssueBudget, error) {
	var budget model.IssueBudget
	err := dbFrom(d.db, tx).WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", budgetID).
		First(&budget).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorw("lock issue budget failed", "budget_id", budgetID, "error", err)
		return nil, err
	}
	return &budget, nil
}

func (d *IssueBudgetDaoImpl) ApplyDistributionDeductAvailable(
	ctx context.Context,
	tx *gorm.DB,
	budgetID int64,
	amount string,
) error {
	ret := dbFrom(d.db, tx).WithContext(ctx).Model(&model.IssueBudget{}).
		Where("id = ? and available_amount >= ?", budgetID, amount).
		Update("available_amount", gorm.Expr("available_amount - ?", amount))
	log.WithContext(ctx).Infof("apply distribution deduct available: budget_id=%d amount=%s rows_affected=%d", budgetID, amount, ret.RowsAffected)
	if ret.Error != nil {
		log.WithContext(ctx).Errorw("deduct issue budget available failed",
			"budget_id", budgetID,
			"amount", amount,
			"error", ret.Error,
		)
		return ret.Error
	}
	if ret.RowsAffected == 0 {
		return errors.New("issue budget available amount is insufficient")
	}
	return nil
}

func (d *IssueBudgetDaoImpl) ApplyDistributionIssued(
	ctx context.Context,
	tx *gorm.DB,
	budgetID int64,
	amount string,
) error {
	err := dbFrom(d.db, tx).WithContext(ctx).Model(&model.IssueBudget{}).
		Where("id = ?", budgetID).
		Update("issued_amount", gorm.Expr("issued_amount + ?", amount)).Error
	if err != nil {
		log.WithContext(ctx).Errorw("apply issue budget issued failed",
			"budget_id", budgetID,
			"amount", amount,
			"error", err,
		)
		return err
	}
	return nil
}

func (d *IssueBudgetDaoImpl) ApplyDistributionRefund(
	ctx context.Context,
	tx *gorm.DB,
	budgetID int64,
	amount string,
) error {
	err := dbFrom(d.db, tx).WithContext(ctx).Model(&model.IssueBudget{}).
		Where("id = ?", budgetID).
		Update("refund_amount", gorm.Expr("refund_amount + ?", amount)).Error
	if err != nil {
		log.WithContext(ctx).Errorw("apply issue budget refund failed",
			"budget_id", budgetID,
			"amount", amount,
			"error", err,
		)
		return err
	}
	return nil
}

var ErrIssueBudgetAlreadyExists = errors.New("issue budget already exists")
