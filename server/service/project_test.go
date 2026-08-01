package service

import (
	"context"
	"testing"

	"github.com/nusiss-capstone-project/reward-mservice/server/config"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao/mocks"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func initServiceTestEnv() {
	config.Config = &config.Conf{
		LogConfig: &config.LogConfig{
			Level:    "debug",
			FilePath: "",
		},
	}
	log.InitLogger()
}

func TestGetProjectServiceSingleton(t *testing.T) {
	initServiceTestEnv()
	s1 := GetProjectService()
	s2 := GetProjectService()
	assert.Same(t, s1, s2)
}

func TestCreateProject(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mocks.ProjectDao)
	svc := &ProjectServiceImpl{projectDao: projectDao}

	projectDao.On("Create", mock.Anything, mock.Anything).Return(int64(10), nil).Once()

	id, err := svc.CreateProject(context.Background(), &data.CreateProjectRequest{
		Name:        "Summer Campaign",
		Description: "Q3 budget",
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(10), id)
}

func TestCreateProjectValidation(t *testing.T) {
	initServiceTestEnv()
	svc := &ProjectServiceImpl{projectDao: new(mocks.ProjectDao)}

	_, err := svc.CreateProject(context.Background(), &data.CreateProjectRequest{Name: " "})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestListProjects(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mocks.ProjectDao)
	svc := &ProjectServiceImpl{projectDao: projectDao}

	projectDao.On("List", mock.Anything, 1, 20).Return([]*model.Project{
		{ID: 1, Name: "P1", Description: "D1"},
	}, int64(1), nil).Once()

	result, err := svc.ListProjects(context.Background(), 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	items := result.Items.([]*data.ProjectVO)
	assert.Len(t, items, 1)
	assert.Equal(t, "P1", items[0].Name)
}

func TestGetProjectByIDNotFound(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mocks.ProjectDao)
	svc := &ProjectServiceImpl{projectDao: projectDao}

	projectDao.On("GetByID", mock.Anything, int64(99)).Return(nil, nil).Once()

	_, err := svc.GetProjectByID(context.Background(), 99)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeProjectNotFound, appErr.Code)
}

func TestGetProjectByIDSuccess(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mocks.ProjectDao)
	svc := &ProjectServiceImpl{projectDao: projectDao}

	projectDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.Project{ID: 1, Name: "P1", Description: "D1"}, nil).Once()

	vo, err := svc.GetProjectByID(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "P1", vo.Name)
}

func TestGetProjectByIDInvalidID(t *testing.T) {
	initServiceTestEnv()
	svc := &ProjectServiceImpl{projectDao: new(mocks.ProjectDao)}

	_, err := svc.GetProjectByID(context.Background(), 0)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestListProjectsInvalidPagination(t *testing.T) {
	initServiceTestEnv()
	svc := &ProjectServiceImpl{projectDao: new(mocks.ProjectDao)}

	_, err := svc.ListProjects(context.Background(), 0, 20)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidPagination, appErr.Code)
}

func TestListProjectsWithOngoingIssueRequest(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mocks.ProjectDao)
	issueRequestDao := new(mocks.IssueRequestDao)
	svc := &ProjectServiceImpl{
		projectDao:      projectDao,
		issueRequestDao: issueRequestDao,
	}

	issueRequestDao.On("ListDistinctProjectIDsByStatus", mock.Anything, model.IssueRequestStatusOngoing).
		Return([]int64{2}, nil).Once()
	projectDao.On("ListByIDs", mock.Anything, []int64{2}).Return([]*model.Project{
		{ID: 2, Name: "Ongoing Project", Description: "has ongoing issue request"},
	}, nil).Once()

	items, err := svc.ListProjectsWithOngoingIssueRequest(context.Background())
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "Ongoing Project", items[0].Name)
}

func TestListProjectsWithOngoingIssueRequestEmpty(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mocks.ProjectDao)
	issueRequestDao := new(mocks.IssueRequestDao)
	svc := &ProjectServiceImpl{
		projectDao:      projectDao,
		issueRequestDao: issueRequestDao,
	}

	issueRequestDao.On("ListDistinctProjectIDsByStatus", mock.Anything, model.IssueRequestStatusOngoing).
		Return([]int64{}, nil).Once()

	items, err := svc.ListProjectsWithOngoingIssueRequest(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, items)
	projectDao.AssertNotCalled(t, "ListByIDs", mock.Anything, mock.Anything)
}
