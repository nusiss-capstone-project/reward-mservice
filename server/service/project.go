package service

import (
	"context"
	"strings"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
)

const defaultCreator = "admin"

type ProjectService interface {
	CreateProject(ctx context.Context, req *data.CreateProjectRequest) (int64, error)
	ListProjects(ctx context.Context, page, size int) (*data.PageResult, error)
	GetProjectByID(ctx context.Context, id int64) (*data.ProjectVO, error)
}

type ProjectServiceImpl struct {
	projectDao dao.ProjectDao
}

var (
	projectServiceOnce sync.Once
	projectServiceInst ProjectService
)

func GetProjectService() ProjectService {
	projectServiceOnce.Do(func() {
		projectServiceInst = &ProjectServiceImpl{projectDao: dao.GetProjectDao()}
	})
	return projectServiceInst
}

func (s *ProjectServiceImpl) CreateProject(ctx context.Context, req *data.CreateProjectRequest) (int64, error) {
	logger := log.WithContext(ctx)
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return 0, errs.New(errs.CodeInvalidRequest, "project name is required")
	}

	project := &model.Project{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
	}
	id, err := s.projectDao.Create(ctx, project)
	if err != nil {
		logger.Errorf("create project failed: %v", err)
		return 0, errs.Wrap(errs.CodeInternalError, err)
	}
	logger.Infof("project created: id=%d", id)
	return id, nil
}

func (s *ProjectServiceImpl) ListProjects(ctx context.Context, page, size int) (*data.PageResult, error) {
	logger := log.WithContext(ctx)
	if page <= 0 || size <= 0 {
		return nil, errs.New(errs.CodeInvalidPagination, "")
	}

	projects, total, err := s.projectDao.List(ctx, page, size)
	if err != nil {
		logger.Errorf("list projects failed: %v", err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}

	items := make([]*data.ProjectVO, 0, len(projects))
	for _, p := range projects {
		items = append(items, toProjectVO(p))
	}
	return &data.PageResult{
		Total: total,
		Page:  page,
		Size:  size,
		Items: items,
	}, nil
}

func (s *ProjectServiceImpl) GetProjectByID(ctx context.Context, id int64) (*data.ProjectVO, error) {
	logger := log.WithContext(ctx)
	if id <= 0 {
		return nil, errs.New(errs.CodeInvalidRequest, "invalid project id")
	}

	project, err := s.projectDao.GetByID(ctx, id)
	if err != nil {
		logger.Errorf("get project failed: id=%d err=%v", id, err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	if project == nil {
		return nil, errs.New(errs.CodeProjectNotFound, "")
	}
	return toProjectVO(project), nil
}

func toProjectVO(project *model.Project) *data.ProjectVO {
	if project == nil {
		return nil
	}
	return &data.ProjectVO{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
	}
}
