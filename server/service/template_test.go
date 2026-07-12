package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockTemplateDao struct {
	mock.Mock
}

func (m *mockTemplateDao) Create(ctx context.Context, template *model.Template) (int64, error) {
	args := m.Called(ctx, template)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockTemplateDao) GetByID(ctx context.Context, id int64) (*model.Template, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Template), args.Error(1)
}

func (m *mockTemplateDao) List(ctx context.Context, page, size int) ([]*model.Template, int64, error) {
	args := m.Called(ctx, page, size)
	return args.Get(0).([]*model.Template), args.Get(1).(int64), args.Error(2)
}

func (m *mockTemplateDao) Update(ctx context.Context, id int64, config []byte) error {
	args := m.Called(ctx, id, config)
	return args.Error(0)
}

func (m *mockTemplateDao) UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus string) error {
	args := m.Called(ctx, id, fromStatus, toStatus)
	return args.Error(0)
}

func newTemplateService(templateDao *mockTemplateDao) *TemplateServiceImpl {
	return &TemplateServiceImpl{templateDao: templateDao}
}

func TestCreateTemplateFixedSuccess(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("Create", mock.Anything, mock.MatchedBy(func(t *model.Template) bool {
		return t.VoucherType == util.VoucherTypeCrypto &&
			t.Unit == util.UnitCryptoUSDT &&
			t.Type == model.TemplateTypeFixed &&
			t.Status == model.TemplateStatusDraft
	})).Return(int64(1), nil).Once()

	id, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "fix",
		Config:      json.RawMessage(`{"amount":"100"}`),
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), id)
}

func TestCreateTemplateDynamicSuccess(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("Create", mock.Anything, mock.MatchedBy(func(t *model.Template) bool {
		return t.Type == model.TemplateTypeDynamic
	})).Return(int64(2), nil).Once()

	id, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "dynamic",
		Config:      json.RawMessage(`{"base_metric":"net_deposit","rate":0.1,"cap":"100"}`),
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), id)
}

func TestCreateTemplateFixedNumericAmount(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("Create", mock.Anything, mock.AnythingOfType("*model.Template")).Return(int64(3), nil).Once()

	id, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "FIXED",
		Config:      json.RawMessage(`{"amount":100}`),
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(3), id)
}

func TestCreateTemplateInvalidType(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "unknown",
		Config:      json.RawMessage(`{"amount":"1"}`),
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestCreateTemplateDynamicMissingBaseMetric(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "DYNAMIC",
		Config:      json.RawMessage(`{"rate":0.1}`),
	})
	assert.Error(t, err)
}

func TestUpdateTemplateSuccess(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	fixConfig, _ := json.Marshal(model.FixTemplateConfig{Amount: "10.00000000"})
	template := &model.Template{
		ID:          1,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        model.TemplateTypeFixed,
		Config:      fixConfig,
		Status:      model.TemplateStatusDraft,
	}

	templateDao.On("GetByID", mock.Anything, int64(1)).Return(template, nil).Once()
	templateDao.On("Update", mock.Anything, int64(1), mock.AnythingOfType("[]uint8")).Return(nil).Once()

	vo, err := svc.UpdateTemplate(context.Background(), 1, &data.UpdateTemplateRequest{
		Config: json.RawMessage(`{"amount":"20"}`),
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), vo.ID)
	fixVO, ok := vo.Config.(data.FixTemplateConfigVO)
	assert.True(t, ok)
	assert.Equal(t, "20.00000000", fixVO.Amount)
}

func TestUpdateTemplateNotEditable(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.Template{ID: 1, Status: model.TemplateStatusPublished}, nil).Once()

	_, err := svc.UpdateTemplate(context.Background(), 1, &data.UpdateTemplateRequest{
		Config: json.RawMessage(`{"amount":"20"}`),
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidStatusTransition, appErr.Code)
}

func TestPublishTemplateSuccess(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.Template{ID: 1, Status: model.TemplateStatusDraft}, nil).Once()
	templateDao.On("UpdateStatus", mock.Anything, int64(1),
		model.TemplateStatusDraft, model.TemplateStatusPublished).Return(nil).Once()

	resp, err := svc.PublishTemplate(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), resp.TemplateID)
	assert.Equal(t, model.TemplateStatusPublished, resp.Status)
}

func TestPublishTemplateInvalidTransition(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.Template{ID: 1, Status: model.TemplateStatusPublished}, nil).Once()

	_, err := svc.PublishTemplate(context.Background(), 1)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidStatusTransition, appErr.Code)
}

func TestPublishTemplateNotFound(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(99)).Return(nil, nil).Once()

	_, err := svc.PublishTemplate(context.Background(), 99)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeTemplateNotFound, appErr.Code)
}

func TestListTemplates(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	fixConfig, _ := json.Marshal(model.FixTemplateConfig{Amount: "1.00000000"})
	templateDao.On("List", mock.Anything, 1, 20).Return([]*model.Template{
		{
			ID:          1,
			VoucherType: util.VoucherTypeCrypto,
			Unit:        util.UnitCryptoUSDT,
			Type:        model.TemplateTypeFixed,
			Config:      fixConfig,
			Status:      model.TemplateStatusDraft,
		},
	}, int64(1), nil).Once()

	result, err := svc.ListTemplates(context.Background(), 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	items := result.Items.([]*data.TemplateVO)
	assert.Len(t, items, 1)
}

func TestGetTemplateServiceSingleton(t *testing.T) {
	initServiceTestEnv()
	s1 := GetTemplateService()
	s2 := GetTemplateService()
	assert.Same(t, s1, s2)
}
