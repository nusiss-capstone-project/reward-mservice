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

type RewardRequestDao interface {
	Create(ctx context.Context, tx *gorm.DB, request *model.RewardRequest) error
	GetByClientRefID(ctx context.Context, clientRefID string) (*model.RewardRequest, error)
	GetByID(ctx context.Context, id int64) (*model.RewardRequest, error)
	UpdateStatus(ctx context.Context, tx *gorm.DB, id int64, status string) error
}

var (
	rewardRequestDao  RewardRequestDao
	rewardRequestOnce sync.Once
)

func GetRewardRequestDao() RewardRequestDao {
	rewardRequestOnce.Do(func() {
		rewardRequestDao = &rewardRequestDaoImpl{db: repository.DB}
	})
	return rewardRequestDao
}

type rewardRequestDaoImpl struct {
	db *gorm.DB
}

func (d *rewardRequestDaoImpl) Create(ctx context.Context, tx *gorm.DB, request *model.RewardRequest) error {
	err := dbFrom(d.db, tx).WithContext(ctx).Create(request).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrDuplicateClientRefID
		}
		log.WithContext(ctx).Errorw("create reward request failed",
			"client_ref_id", request.ClientRefID,
			"error", err,
		)
		return err
	}
	log.WithContext(ctx).Infow("reward request created",
		"reward_request_id", request.ID,
		"client_ref_id", request.ClientRefID,
	)
	return nil
}

func (d *rewardRequestDaoImpl) GetByClientRefID(ctx context.Context, clientRefID string) (*model.RewardRequest, error) {
	var request model.RewardRequest
	err := d.db.WithContext(ctx).Where("client_ref_id = ?", clientRefID).First(&request).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorw("get reward request by client_ref_id failed",
			"client_ref_id", clientRefID,
			"error", err,
		)
		return nil, err
	}
	return &request, nil
}

func (d *rewardRequestDaoImpl) GetByID(ctx context.Context, id int64) (*model.RewardRequest, error) {
	var request model.RewardRequest
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&request).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorw("get reward request failed", "reward_request_id", id, "error", err)
		return nil, err
	}
	return &request, nil
}

func (d *rewardRequestDaoImpl) UpdateStatus(ctx context.Context, tx *gorm.DB, id int64, status string) error {
	err := dbFrom(d.db, tx).WithContext(ctx).
		Model(&model.RewardRequest{}).
		Where("id = ?", id).
		Update("status", status).Error
	if err != nil {
		log.WithContext(ctx).Errorw("update reward request status failed",
			"reward_request_id", id,
			"status", status,
			"error", err,
		)
		return err
	}
	log.WithContext(ctx).Infow("reward request status updated",
		"reward_request_id", id,
		"status", status,
	)
	return nil
}

var ErrDuplicateClientRefID = errors.New("duplicate client_ref_id")
