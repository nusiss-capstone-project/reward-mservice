package dao

import (
	"context"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
)

type TemplateDao interface {
	Create(ctx context.Context, template *model.Template) (int64, error)
	GetByID(ctx context.Context, id int64) (*model.Template, error)
	List(ctx context.Context, page, size int) ([]*model.Template, int64, error)
	Update(ctx context.Context, id int64, config []byte) error
	UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus string) error
}

type TemplateDaoImpl struct {
	db *gorm.DB
}

var (
	templateOnce sync.Once
	templateDao  TemplateDao
)

func GetTemplateDao() TemplateDao {
	templateOnce.Do(func() {
		templateDao = &TemplateDaoImpl{db: repository.DB}
	})
	return templateDao
}

func (d *TemplateDaoImpl) Create(ctx context.Context, template *model.Template) (int64, error) {
	if err := d.db.WithContext(ctx).Create(template).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to create template: %v", err)
		return 0, err
	}
	return template.ID, nil
}

func (d *TemplateDaoImpl) GetByID(ctx context.Context, id int64) (*model.Template, error) {
	var template model.Template
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&template).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		log.WithContext(ctx).Errorf("failed to get template by id: %v", err)
		return nil, err
	}
	return &template, nil
}

func (d *TemplateDaoImpl) List(ctx context.Context, page, size int) ([]*model.Template, int64, error) {
	var total int64
	query := d.db.WithContext(ctx).Model(&model.Template{})
	if err := query.Count(&total).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to count templates: %v", err)
		return nil, 0, err
	}

	offset := (page - 1) * size
	var templates []*model.Template
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&templates).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to list templates: %v", err)
		return nil, 0, err
	}
	return templates, total, nil
}

func (d *TemplateDaoImpl) Update(ctx context.Context, id int64, config []byte) error {
	if err := d.db.WithContext(ctx).Model(&model.Template{}).
		Where("id = ?", id).
		Update("config", config).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to update template config: %v", err)
		return err
	}
	return nil
}

func (d *TemplateDaoImpl) UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus string) error {
	result := d.db.WithContext(ctx).Model(&model.Template{}).
		Where("id = ? AND status = ?", id, fromStatus).
		Update("status", toStatus)
	if result.Error != nil {
		log.WithContext(ctx).Errorf("failed to update template status: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errs.New(errs.CodeInvalidStatusTransition, "template status transition failed")
	}
	return nil
}
