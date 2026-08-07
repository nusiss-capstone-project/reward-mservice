package dao

import (
	"context"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
)

type FinanceDocDao interface {
	Create(ctx context.Context, doc *model.FinanceDoc) error
	GetByDocID(ctx context.Context, docID string) (*model.FinanceDoc, error)
	ExistsByProjectID(ctx context.Context, projectID int64) (bool, error)
	List(ctx context.Context, page, size int, status, creator string) ([]*model.FinanceDoc, int64, error)
	UpdateStatus(ctx context.Context, docID, status, remark string) error
	UpdateContent(ctx context.Context, docID, description string, applicationDetail []byte) error
}

type FinanceDocDaoImpl struct {
	db *gorm.DB
}

var (
	financeDocOnce sync.Once
	financeDocDao  FinanceDocDao
)

func GetFinanceDocDao() FinanceDocDao {
	financeDocOnce.Do(func() {
		financeDocDao = &FinanceDocDaoImpl{db: repository.DB}
	})
	return financeDocDao
}

func (d *FinanceDocDaoImpl) Create(ctx context.Context, doc *model.FinanceDoc) error {
	if err := d.db.WithContext(ctx).Create(doc).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to create finance doc: %v", err)
		return err
	}
	return nil
}

func (d *FinanceDocDaoImpl) GetByDocID(ctx context.Context, docID string) (*model.FinanceDoc, error) {
	var doc model.FinanceDoc
	err := d.db.WithContext(ctx).Where("doc_id = ?", docID).First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		log.WithContext(ctx).Errorf("failed to get finance doc: %v", err)
		return nil, err
	}
	return &doc, nil
}

func (d *FinanceDocDaoImpl) ExistsByProjectID(ctx context.Context, projectID int64) (bool, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&model.FinanceDoc{}).
		Where("project_id = ?", projectID).
		Count(&count).Error
	if err != nil {
		log.WithContext(ctx).Errorf("failed to check finance doc by project: %v", err)
		return false, err
	}
	return count > 0, nil
}

func (d *FinanceDocDaoImpl) List(ctx context.Context, page, size int, status, creator string) ([]*model.FinanceDoc, int64, error) {
	var total int64
	query := d.db.WithContext(ctx).Model(&model.FinanceDoc{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if creator != "" {
		query = query.Where("creator = ?", creator)
	}
	if err := query.Count(&total).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to count finance docs: %v", err)
		return nil, 0, err
	}

	offset := (page - 1) * size
	var docs []*model.FinanceDoc
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&docs).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to list finance docs: %v", err)
		return nil, 0, err
	}
	return docs, total, nil
}

func (d *FinanceDocDaoImpl) UpdateStatus(ctx context.Context, docID, status, remark string) error {
	updates := map[string]interface{}{
		"status": status,
		"remark": remark,
	}
	if err := d.db.WithContext(ctx).Model(&model.FinanceDoc{}).
		Where("doc_id = ?", docID).
		Updates(updates).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to update finance doc status: %v", err)
		return err
	}
	return nil
}

func (d *FinanceDocDaoImpl) UpdateContent(ctx context.Context, docID, description string, applicationDetail []byte) error {
	updates := map[string]interface{}{
		"description":        description,
		"application_detail": applicationDetail,
	}
	if err := d.db.WithContext(ctx).Model(&model.FinanceDoc{}).
		Where("doc_id = ?", docID).
		Updates(updates).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to update finance doc content: %v", err)
		return err
	}
	return nil
}
