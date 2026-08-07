package dao

import (
	"context"
	"errors"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IssueRequestDao interface {
	Create(ctx context.Context, request *model.IssueRequest) error
	GetByID(ctx context.Context, id int64) (*model.IssueRequest, error)
	GetByIDForUpdate(ctx context.Context, tx *gorm.DB, id int64) (*model.IssueRequest, error)
	ListByProjectID(ctx context.Context, projectID int64, page, size int, status, creator string) ([]*model.IssueRequest, int64, error)
	ListDistinctProjectIDsByStatus(ctx context.Context, status string) ([]int64, error)
	UpdateFields(ctx context.Context, id int64, voucherType, unit, amount, remark string) error
	UpdateStatusInTx(ctx context.Context, tx *gorm.DB, id int64, fromStatus, toStatus, remark string) error
	MarkOngoing(ctx context.Context, tx *gorm.DB, id int64) error
}

type IssueRequestDaoImpl struct {
	db *gorm.DB
}

var (
	issueRequestOnce sync.Once
	issueRequestDao  IssueRequestDao
)

func GetIssueRequestDao() IssueRequestDao {
	issueRequestOnce.Do(func() {
		issueRequestDao = &IssueRequestDaoImpl{db: repository.DB}
	})
	return issueRequestDao
}

func (d *IssueRequestDaoImpl) Create(ctx context.Context, request *model.IssueRequest) error {
	err := d.db.WithContext(ctx).Create(request).Error
	if err != nil {
		log.WithContext(ctx).Errorw("create issue request failed",
			"project_id", request.ProjectID,
			"error", err,
		)
		return err
	}
	return nil
}

func (d *IssueRequestDaoImpl) GetByID(ctx context.Context, id int64) (*model.IssueRequest, error) {
	var request model.IssueRequest
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&request).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorw("get issue request failed", "issue_request_id", id, "error", err)
		return nil, err
	}
	return &request, nil
}

func (d *IssueRequestDaoImpl) GetByIDForUpdate(ctx context.Context, tx *gorm.DB, id int64) (*model.IssueRequest, error) {
	var request model.IssueRequest
	err := dbFrom(d.db, tx).WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
		First(&request).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorw("lock issue request failed", "issue_request_id", id, "error", err)
		return nil, err
	}
	return &request, nil
}

func (d *IssueRequestDaoImpl) ListByProjectID(
	ctx context.Context,
	projectID int64,
	page, size int,
	status, creator string,
) ([]*model.IssueRequest, int64, error) {
	query := d.db.WithContext(ctx).Model(&model.IssueRequest{}).Where("project_id = ?", projectID)
	if status != "" {
		query = query.Where("request_status = ?", status)
	}
	if creator != "" {
		query = query.Where("creator = ?", creator)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		log.WithContext(ctx).Errorw("count issue requests failed", "project_id", projectID, "error", err)
		return nil, 0, err
	}

	offset := (page - 1) * size
	var requests []*model.IssueRequest
	err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&requests).Error
	if err != nil {
		log.WithContext(ctx).Errorw("list issue requests failed", "project_id", projectID, "error", err)
		return nil, 0, err
	}
	return requests, total, nil
}

func (d *IssueRequestDaoImpl) ListDistinctProjectIDsByStatus(ctx context.Context, status string) ([]int64, error) {
	var projectIDs []int64
	err := d.db.WithContext(ctx).Model(&model.IssueRequest{}).
		Where("request_status = ?", status).
		Distinct("project_id").
		Order("project_id DESC").
		Pluck("project_id", &projectIDs).Error
	if err != nil {
		log.WithContext(ctx).Errorw("list distinct project ids by status failed", "status", status, "error", err)
		return nil, err
	}
	return projectIDs, nil
}

func (d *IssueRequestDaoImpl) UpdateFields(
	ctx context.Context,
	id int64,
	voucherType, unit, amount, remark string,
) error {
	err := d.db.WithContext(ctx).Model(&model.IssueRequest{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"voucher_type": voucherType,
			"unit":         unit,
			"amount":       amount,
			"remark":       remark,
		}).Error
	if err != nil {
		log.WithContext(ctx).Errorw("update issue request fields failed", "issue_request_id", id, "error", err)
		return err
	}
	log.WithContext(ctx).Infof("issue request fields updated: issue_request_id=%d voucher_type=%s unit=%s amount=%s remark=%s", id, voucherType, unit, amount, remark)
	return nil
}

func (d *IssueRequestDaoImpl) UpdateStatusInTx(
	ctx context.Context,
	tx *gorm.DB,
	id int64,
	fromStatus, toStatus, remark string,
) error {
	ret := dbFrom(d.db, tx).WithContext(ctx).Model(&model.IssueRequest{}).
		Where("id = ? AND request_status = ?", id, fromStatus).
		Updates(map[string]interface{}{
			"request_status": toStatus,
			"remark":         remark,
		})
	if ret.Error != nil {
		log.WithContext(ctx).Errorw("update issue request status failed", "issue_request_id", id, "error", ret.Error)
		return ret.Error
	}
	if ret.RowsAffected == 0 {
		log.WithContext(ctx).Errorw("issue request status not match", "issue_request_id", id, "current_status", fromStatus, "to_status", toStatus)
		return errs.New(errs.CodeInvalidStatusTransition, "issue request status transition failed")
	}
	log.WithContext(ctx).Infof("issue request status updated: issue_request_id=%d from_status=%s to_status=%s remark=%s", id, fromStatus, toStatus, remark)
	return nil
}

func (d *IssueRequestDaoImpl) MarkOngoing(ctx context.Context, tx *gorm.DB, id int64) error {
	ret := dbFrom(d.db, tx).WithContext(ctx).Model(&model.IssueRequest{}).
		Where("id = ? AND request_status = ?", id, model.IssueRequestStatusApproved).
		Update("request_status", model.IssueRequestStatusOngoing)
	if ret.Error != nil {
		log.WithContext(ctx).Errorw("mark issue request ongoing failed", "issue_request_id", id, "error", ret.Error)
		return ret.Error
	}
	log.WithContext(ctx).Infof("issue request marked ongoing: issue_request_id=%d", id)
	return nil
}
