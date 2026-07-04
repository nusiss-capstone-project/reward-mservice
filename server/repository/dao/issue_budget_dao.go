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

type IssueBudgetDao interface {
	Create(ctx context.Context, tx *gorm.DB, budget *model.IssueBudget) error
	GetByIssueRequestID(ctx context.Context, issueRequestID int64) (*model.IssueBudget, error)
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

var ErrIssueBudgetAlreadyExists = errors.New("issue budget already exists")
