package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao/mocks"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTemplateService(templateDao *mocks.TemplateDao) *TemplateServiceImpl {
	return &TemplateServiceImpl{templateDao: templateDao}
}

func TestCreateTemplateFixedSuccess(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
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
	templateDao := new(mocks.TemplateDao)
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
	templateDao := new(mocks.TemplateDao)
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
	svc := newTemplateService(new(mocks.TemplateDao))

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
	svc := newTemplateService(new(mocks.TemplateDao))

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
	templateDao := new(mocks.TemplateDao)
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
	templateDao := new(mocks.TemplateDao)
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
	templateDao := new(mocks.TemplateDao)
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
	templateDao := new(mocks.TemplateDao)
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
	templateDao := new(mocks.TemplateDao)
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
	templateDao := new(mocks.TemplateDao)
	svc := newTemplateService(templateDao)

	fixConfig, _ := json.Marshal(model.FixTemplateConfig{Amount: "1.00000000"})
	templateDao.On("List", mock.Anything, 1, 20, "").Return([]*model.Template{
		{
			ID:          1,
			VoucherType: util.VoucherTypeCrypto,
			Unit:        util.UnitCryptoUSDT,
			Type:        model.TemplateTypeFixed,
			Config:      fixConfig,
			Status:      model.TemplateStatusDraft,
		},
	}, int64(1), nil).Once()

	result, err := svc.ListTemplates(context.Background(), data.TemplateListQuery{PageQuery: data.PageQuery{Page: 1, Size: 20}})
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
	svc := newTemplateService(new(mocks.TemplateDao))

	_, err := svc.CreateTemplate(context.Background(), nil)
	assert.Error(t, err)
}

func TestCreateTemplateCreateFailed(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
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
	svc := newTemplateService(new(mocks.TemplateDao))

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "FIXED",
	})
	assert.Error(t, err)
}

func TestCreateTemplateInvalidAmount(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mocks.TemplateDao))

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
	svc := newTemplateService(new(mocks.TemplateDao))

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
	svc := newTemplateService(new(mocks.TemplateDao))

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
	svc := newTemplateService(new(mocks.TemplateDao))

	_, err := svc.UpdateTemplate(context.Background(), 1, nil)
	assert.Error(t, err)
}

func TestUpdateTemplateEmptyConfig(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.Template{ID: 1, Type: model.TemplateTypeFixed, Status: model.TemplateStatusDraft}, nil).Once()

	_, err := svc.UpdateTemplate(context.Background(), 1, &data.UpdateTemplateRequest{})
	assert.Error(t, err)
}

func TestUpdateTemplateInvalidID(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mocks.TemplateDao))

	_, err := svc.UpdateTemplate(context.Background(), 0, &data.UpdateTemplateRequest{
		Config: json.RawMessage(`{"amount":"1"}`),
	})
	assert.Error(t, err)
}

func TestUpdateTemplateNotFound(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
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
	templateDao := new(mocks.TemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).Return(nil, assert.AnError).Once()

	_, err := svc.UpdateTemplate(context.Background(), 1, &data.UpdateTemplateRequest{
		Config: json.RawMessage(`{"amount":"1"}`),
	})
	assert.Error(t, err)
}

func TestUpdateTemplateUpdateFailed(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
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
	templateDao := new(mocks.TemplateDao)
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

func TestListTemplatesDefaultPagination(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("List", mock.Anything, data.DefaultPage, data.DefaultSize, "").
		Return([]*model.Template{}, int64(0), nil).Once()

	result, err := svc.ListTemplates(context.Background(), data.TemplateListQuery{})
	assert.NoError(t, err)
	assert.Equal(t, data.DefaultPage, result.Page)
	assert.Equal(t, data.DefaultSize, result.Size)
}

func TestListTemplatesByStatus(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newTemplateService(templateDao)

	fixConfig, _ := json.Marshal(model.FixTemplateConfig{Amount: "1.00000000"})
	templateDao.On("List", mock.Anything, 1, 20, model.TemplateStatusPublished).Return([]*model.Template{
		{
			ID:     3,
			Type:   model.TemplateTypeFixed,
			Config: fixConfig,
			Status: model.TemplateStatusPublished,
		},
	}, int64(1), nil).Once()

	result, err := svc.ListTemplates(context.Background(), data.TemplateListQuery{PageQuery: data.PageQuery{Page: 1, Size: 20}, Status: "published"})
	assert.NoError(t, err)
	items := result.Items.([]*data.TemplateVO)
	assert.Len(t, items, 1)
	assert.Equal(t, model.TemplateStatusPublished, items[0].Status)
}

func TestListTemplatesInvalidStatus(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mocks.TemplateDao))

	_, err := svc.ListTemplates(context.Background(), data.TemplateListQuery{PageQuery: data.PageQuery{Page: 1, Size: 20}, Status: "UNKNOWN"})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestListTemplatesFailed(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("List", mock.Anything, 1, 20, "").Return([]*model.Template(nil), int64(0), assert.AnError).Once()

	_, err := svc.ListTemplates(context.Background(), data.TemplateListQuery{PageQuery: data.PageQuery{Page: 1, Size: 20}})
	assert.Error(t, err)
}

func TestListTemplatesDynamic(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newTemplateService(templateDao)

	dynamicConfig, _ := json.Marshal(model.DynamicTemplateConfig{
		BaseMetric: "net_deposit", Rate: 0.1, Cap: "100.00000000",
	})
	templateDao.On("List", mock.Anything, 1, 20, "").Return([]*model.Template{
		{ID: 2, Type: model.TemplateTypeDynamic, Config: dynamicConfig, Status: model.TemplateStatusDraft},
	}, int64(1), nil).Once()

	result, err := svc.ListTemplates(context.Background(), data.TemplateListQuery{PageQuery: data.PageQuery{Page: 1, Size: 20}})
	assert.NoError(t, err)
	items := result.Items.([]*data.TemplateVO)
	dynamicVO, ok := items[0].Config.(data.DynamicTemplateConfigVO)
	assert.True(t, ok)
	assert.Equal(t, "net_deposit", dynamicVO.BaseMetric)
}

func TestPublishTemplateGetByIDFailed(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("GetByID", mock.Anything, int64(1)).Return(nil, assert.AnError).Once()

	_, err := svc.PublishTemplate(context.Background(), 1)
	assert.Error(t, err)
}

func TestPublishTemplateStatusTransitionConflict(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
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
	templateDao := new(mocks.TemplateDao)
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
	svc := newTemplateService(new(mocks.TemplateDao))

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
	svc := newTemplateService(new(mocks.TemplateDao))

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
	templateDao := new(mocks.TemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("List", mock.Anything, 1, 20, "").Return([]*model.Template{
		{ID: 3, Type: "UNKNOWN", Config: []byte(`{}`), Status: model.TemplateStatusDraft},
	}, int64(1), nil).Once()

	_, err := svc.ListTemplates(context.Background(), data.TemplateListQuery{PageQuery: data.PageQuery{Page: 1, Size: 20}})
	assert.Error(t, err)
}

func TestCreateTemplateFixedRejectsDynamicFields(t *testing.T) {
	initServiceTestEnv()
	svc := newTemplateService(new(mocks.TemplateDao))

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "FIXED",
		Config:      json.RawMessage(`{"amount":"100","base_metric":"net_deposit"}`),
	})
	assert.Error(t, err)
}

func TestCreateTemplateFixedStoresOnlyAmount(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newTemplateService(templateDao)

	templateDao.On("Create", mock.Anything, mock.MatchedBy(func(t *model.Template) bool {
		var cfg model.FixTemplateConfig
		if err := json.Unmarshal(t.Config, &cfg); err != nil {
			return false
		}
		return cfg.Amount == "100.00000000" && string(t.Config) == `{"amount":"100.00000000"}`
	})).Return(int64(1), nil).Once()

	_, err := svc.CreateTemplate(context.Background(), &data.CreateTemplateRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        "FIXED",
		Config:      json.RawMessage(`{"amount":"100"}`),
	})
	assert.NoError(t, err)
}
