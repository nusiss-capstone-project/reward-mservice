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

type IssueRecordDao interface {
	Create(ctx context.Context, tx *gorm.DB, issueRecord *model.IssueRecord) error
	Save(ctx context.Context, tx *gorm.DB, issueRecord *model.IssueRecord) error
	GetByID(ctx context.Context, id int64) (*model.IssueRecord, error)
	GetByClientRefId(ctx context.Context, clientRefID string) (*model.IssueRecord, error)
	ListByProjectIDAndUserID(ctx context.Context, projectID, userID int64) ([]*model.IssueRecord, error)
}

var (
	issueRecordDao  IssueRecordDao
	issueRecordOnce sync.Once
)

func GetIssueRecordDao() IssueRecordDao {
	issueRecordOnce.Do(func() {
		issueRecordDao = &issueRecordDaoImpl{
			db: repository.DB,
		}
	})
	return issueRecordDao
}

type issueRecordDaoImpl struct {
	db *gorm.DB
}

func (d *issueRecordDaoImpl) Create(ctx context.Context, tx *gorm.DB, issueRecord *model.IssueRecord) error {
	err := dbFrom(d.db, tx).WithContext(ctx).Create(issueRecord).Error
	if err != nil {
		log.WithContext(ctx).Errorw("create issue record failed",
			"reward_request_id", issueRecord.RewardRequestID,
			"error", err,
		)
		return err
	}
	log.WithContext(ctx).Infow("issue record created",
		"reward_request_id", issueRecord.RewardRequestID,
		"voucher_id", issueRecord.VoucherID,
	)
	return nil
}

func (d *issueRecordDaoImpl) Save(ctx context.Context, tx *gorm.DB, issueRecord *model.IssueRecord) error {
	err := dbFrom(d.db, tx).WithContext(ctx).Save(issueRecord).Error
	if err != nil {
		log.WithContext(ctx).Errorw("save issue record failed",
			"issue_record_id", issueRecord.ID,
			"error", err,
		)
		return err
	}
	log.WithContext(ctx).Infow("issue record saved",
		"issue_record_id", issueRecord.ID,
		"voucher_id", issueRecord.VoucherID,
		"issue_status", issueRecord.IssueStatus,
	)
	return nil
}

func (d *issueRecordDaoImpl) GetByID(ctx context.Context, id int64) (*model.IssueRecord, error) {
	var record model.IssueRecord
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorw("get issue record by id failed", "issue_record_id", id, "error", err)
		return nil, err
	}
	return &record, nil
}

func (d *issueRecordDaoImpl) GetByClientRefId(ctx context.Context, clientRefID string) (*model.IssueRecord, error) {
	var record model.IssueRecord
	err := d.db.WithContext(ctx).Where("client_reference_id = ?", clientRefID).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorw("get issue record by client_ref_id failed", "client_ref_id", clientRefID, "error", err)
		return nil, err
	}
	return &record, nil
}

func (d *issueRecordDaoImpl) ListByProjectIDAndUserID(
	ctx context.Context,
	projectID, userID int64,
) ([]*model.IssueRecord, error) {
	var records []*model.IssueRecord
	err := d.db.WithContext(ctx).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Order("created_at DESC").
		Find(&records).Error
	if err != nil {
		log.WithContext(ctx).Errorw("list issue records by project and user failed",
			"project_id", projectID,
			"user_id", userID,
			"error", err,
		)
		return nil, err
	}
	return records, nil
}
