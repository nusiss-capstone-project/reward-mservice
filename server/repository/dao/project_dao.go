package dao

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/cache"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
)

const (
	projectCacheTTL      = 5 * time.Minute
	projectCacheCleanup  = 10 * time.Minute
	projectCacheKeyFmt   = "project:%d"
)

type ProjectDao interface {
	Create(ctx context.Context, project *model.Project) (int64, error)
	GetByID(ctx context.Context, id int64) (*model.Project, error)
	List(ctx context.Context, page, size int) ([]*model.Project, int64, error)
	ListByIDs(ctx context.Context, ids []int64) ([]*model.Project, error)
}

type ProjectDaoImpl struct {
	db    *gorm.DB
	cache *cache.Cache[*model.Project]
}

var (
	projectOnce sync.Once
	projectDao  ProjectDao
)

func GetProjectDao() ProjectDao {
	projectOnce.Do(func() {
		projectDao = &ProjectDaoImpl{
			db:    repository.DB,
			cache: cache.NewCache[*model.Project](projectCacheTTL, projectCacheCleanup),
		}
	})
	return projectDao
}

func projectCacheKey(id int64) string {
	return fmt.Sprintf(projectCacheKeyFmt, id)
}

func (d *ProjectDaoImpl) Create(ctx context.Context, project *model.Project) (int64, error) {
	if err := d.db.WithContext(ctx).Create(project).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to create project: %v", err)
		return 0, err
	}
	d.cache.Set(projectCacheKey(project.ID), project)
	return project.ID, nil
}

func (d *ProjectDaoImpl) GetByID(ctx context.Context, id int64) (*model.Project, error) {
	return d.cache.GetWithLoad(ctx, projectCacheKey(id), func(ctx context.Context) (*model.Project, error) {
		return d.getByIDFromDB(ctx, id)
	})
}

func (d *ProjectDaoImpl) getByIDFromDB(ctx context.Context, id int64) (*model.Project, error) {
	var project model.Project
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&project).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		log.WithContext(ctx).Errorf("failed to get project by id: %v", err)
		return nil, err
	}
	return &project, nil
}

func (d *ProjectDaoImpl) List(ctx context.Context, page, size int) ([]*model.Project, int64, error) {
	var total int64
	query := d.db.WithContext(ctx).Model(&model.Project{})
	if err := query.Count(&total).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to count projects: %v", err)
		return nil, 0, err
	}

	offset := (page - 1) * size
	var projects []*model.Project
	if err := query.Order("id DESC").Offset(offset).Limit(size).Find(&projects).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to list projects: %v", err)
		return nil, 0, err
	}
	return projects, total, nil
}

func (d *ProjectDaoImpl) ListByIDs(ctx context.Context, ids []int64) ([]*model.Project, error) {
	if len(ids) == 0 {
		return []*model.Project{}, nil
	}

	var projects []*model.Project
	if err := d.db.WithContext(ctx).Where("id IN ?", ids).Find(&projects).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to list projects by ids: %v", err)
		return nil, err
	}

	byID := make(map[int64]*model.Project, len(projects))
	for _, project := range projects {
		byID[project.ID] = project
	}
	ordered := make([]*model.Project, 0, len(ids))
	for _, id := range ids {
		if project, ok := byID[id]; ok {
			ordered = append(ordered, project)
		}
	}
	return ordered, nil
}
