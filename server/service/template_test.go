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

func TestCreateTemplateNilRequest(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.CreateTemplate(context.Background(), nil)
	assert.Error(t, err)
}

func TestCreateTemplateCreateFailed(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("Create", mock.Anything, mock.AnythingOfType("*model.Template")).
		Return(int64(0), assert.AnError).Once()

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "FIXED",
		Config:      json.RawMessage(`{"amount":"100"}`),
	})
	assert.Error(t, err)
}

func TestCreateTemplateEmptyConfig(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "FIXED",
	})
	assert.Error(t, err)
}

func TestCreateTemplateInvalidAmount(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "FIXED",
		Config:      json.RawMessage(`{"amount":"bad"}`),
	})
	assert.Error(t, err)
}

func TestCreateTemplateDynamicInvalidRate(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "DYNAMIC",
		Config:      json.RawMessage(`{"base_metric":"net_deposit","rate":0}`),
	})
	assert.Error(t, err)
}

func TestCreateTemplateDynamicInvalidCap(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "DYNAMIC",
		Config:      json.RawMessage(`{"base_metric":"net_deposit","rate":0.1,"cap":"bad"}`),
	})
	assert.Error(t, err)
}

func TestUpdateTemplateNilRequest(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.UpdateTemplate(context.Background(), 1, nil)
	assert.Error(t, err)
}

func TestUpdateTemplateEmptyConfig(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.Template{ID: 1, Type: model.TemplateTypeFixed, Status: model.TemplateStatusDraft}, nil).Once()

	_, err := svc.UpdateTemplate(context.Background(), 1, &data.UpdateTemplateRequest{})
	assert.Error(t, err)
}

func TestUpdateTemplateInvalidID(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.UpdateTemplate(context.Background(), 0, &data.UpdateTemplateRequest{
		Config: json.RawMessage(`{"amount":"1"}`),
	})
	assert.Error(t, err)
}

func TestUpdateTemplateNotFound(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(99)).Return(nil, nil).Once()

	_, err := svc.UpdateTemplate(context.Background(), 99, &data.UpdateTemplateRequest{
		Config: json.RawMessage(`{"amount":"1"}`),
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeTemplateNotFound, appErr.Code)
}

func TestUpdateTemplateGetByIDFailed(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).Return(nil, assert.AnError).Once()

	_, err := svc.UpdateTemplate(context.Background(), 1, &data.UpdateTemplateRequest{
		Config: json.RawMessage(`{"amount":"1"}`),
	})
	assert.Error(t, err)
}

func TestUpdateTemplateUpdateFailed(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.Template{ID: 1, Type: model.TemplateTypeFixed, Status: model.TemplateStatusDraft}, nil).Once()
	templateDao.On("Update", mock.Anything, int64(1), mock.AnythingOfType("[]uint8")).
		Return(assert.AnError).Once()

	_, err := svc.UpdateTemplate(context.Background(), 1, &data.UpdateTemplateRequest{
		Config: json.RawMessage(`{"amount":"20"}`),
	})
	assert.Error(t, err)
}

func TestUpdateTemplateDynamicSuccess(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	template := &model.Template{
		ID:     1,
		Type:   model.TemplateTypeDynamic,
		Status: model.TemplateStatusDraft,
		Config: []byte(`{"base_metric":"old","rate":0.1}`),
	}
	templateDao.On("GetByID", mock.Anything, int64(1)).Return(template, nil).Once()
	templateDao.On("Update", mock.Anything, int64(1), mock.AnythingOfType("[]uint8")).Return(nil).Once()

	vo, err := svc.UpdateTemplate(context.Background(), 1, &data.UpdateTemplateRequest{
		Config: json.RawMessage(`{"base_metric":"net_deposit","rate":0.2,"cap":"50"}`),
	})
	assert.NoError(t, err)
	dynamicVO, ok := vo.Config.(data.DynamicTemplateConfigVO)
	assert.True(t, ok)
	assert.Equal(t, "net_deposit", dynamicVO.BaseMetric)
	assert.Equal(t, 0.2, dynamicVO.Rate)
}

func TestListTemplatesInvalidPagination(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.ListTemplates(context.Background(), 0, 20)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidPagination, appErr.Code)
}

func TestListTemplatesFailed(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("List", mock.Anything, 1, 20).Return([]*model.Template(nil), int64(0), assert.AnError).Once()

	_, err := svc.ListTemplates(context.Background(), 1, 20)
	assert.Error(t, err)
}

func TestListTemplatesDynamic(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	dynamicConfig, _ := json.Marshal(model.DynamicTemplateConfig{
		BaseMetric: "net_deposit", Rate: 0.1, Cap: "100.00000000",
	})
	templateDao.On("List", mock.Anything, 1, 20).Return([]*model.Template{
		{ID: 2, Type: model.TemplateTypeDynamic, Config: dynamicConfig, Status: model.TemplateStatusDraft},
	}, int64(1), nil).Once()

	result, err := svc.ListTemplates(context.Background(), 1, 20)
	assert.NoError(t, err)
	items := result.Items.([]*data.TemplateVO)
	dynamicVO, ok := items[0].Config.(data.DynamicTemplateConfigVO)
	assert.True(t, ok)
	assert.Equal(t, "net_deposit", dynamicVO.BaseMetric)
}

func TestPublishTemplateGetByIDFailed(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).Return(nil, assert.AnError).Once()

	_, err := svc.PublishTemplate(context.Background(), 1)
	assert.Error(t, err)
}

func TestPublishTemplateStatusTransitionConflict(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.Template{ID: 1, Status: model.TemplateStatusDraft}, nil).Once()
	templateDao.On("UpdateStatus", mock.Anything, int64(1),
		model.TemplateStatusDraft, model.TemplateStatusPublished).
		Return(errs.New(errs.CodeInvalidStatusTransition, "template status transition failed")).Once()

	_, err := svc.PublishTemplate(context.Background(), 1)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidStatusTransition, appErr.Code)
}

func TestPublishTemplateUpdateStatusFailed(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.Template{ID: 1, Status: model.TemplateStatusDraft}, nil).Once()
	templateDao.On("UpdateStatus", mock.Anything, int64(1),
		model.TemplateStatusDraft, model.TemplateStatusPublished).
		Return(assert.AnError).Once()

	_, err := svc.PublishTemplate(context.Background(), 1)
	assert.Error(t, err)
}

func TestParseTemplateID(t *testing.T) {
	initServiceTestEnv()

	_, err := ParseTemplateID(" ")
	assert.Error(t, err)

	_, err = ParseTemplateID("bad")
	assert.Error(t, err)

	id, err := ParseTemplateID(" 10 ")
	assert.NoError(t, err)
	assert.Equal(t, int64(10), id)
}

func TestCreateTemplateInvalidVoucherType(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: "BAD",
		Unit:        util.UnitCryptoUSDT,
		Type:        "FIXED",
		Config:      json.RawMessage(`{"amount":"1"}`),
	})
	assert.Error(t, err)
}

func TestCreateTemplateFixedMissingAmount(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mockTemplateDao))

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "FIXED",
		Config:      json.RawMessage(`{}`),
	})
	assert.Error(t, err)
}

func TestListTemplatesInvalidStoredType(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mockTemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("List", mock.Anything, 1, 20).Return([]*model.Template{
		{ID: 3, Type: "UNKNOWN", Config: []byte(`{}`), Status: model.TemplateStatusDraft},
	}, int64(1), nil).Once()

	_, err := svc.ListTemplates(context.Background(), 1, 20)
	assert.Error(t, err)
}
