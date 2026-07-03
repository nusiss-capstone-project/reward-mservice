package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockFinanceDocDao struct {
	mock.Mock
}

func (m *mockFinanceDocDao) Create(ctx context.Context, doc *model.FinanceDoc) error {
	args := m.Called(ctx, doc)
	return args.Error(0)
}

func (m *mockFinanceDocDao) GetByDocID(ctx context.Context, docID string) (*model.FinanceDoc, error) {
	args := m.Called(ctx, docID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.FinanceDoc), args.Error(1)
}

func (m *mockFinanceDocDao) List(ctx context.Context, page, size int) ([]*model.FinanceDoc, int64, error) {
	args := m.Called(ctx, page, size)
	return args.Get(0).([]*model.FinanceDoc), args.Get(1).(int64), args.Error(2)
}

func (m *mockFinanceDocDao) UpdateStatus(ctx context.Context, docID, status, remark string) error {
	args := m.Called(ctx, docID, status, remark)
	return args.Error(0)
}

type mockPaymentConfigDao struct {
	mock.Mock
}

func (m *mockPaymentConfigDao) ListAll(ctx context.Context) ([]*model.PaymentConfig, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*model.PaymentConfig), args.Error(1)
}

func (m *mockPaymentConfigDao) FindExistingPayAddresses(ctx context.Context, payAddresses []string) (map[string]struct{}, error) {
	args := m.Called(ctx, payAddresses)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]struct{}), args.Error(1)
}

func newFinanceDocService(projectDao *mockProjectDao, financeDocDao *mockFinanceDocDao, paymentConfigDao *mockPaymentConfigDao) *FinanceDocServiceImpl {
	return &FinanceDocServiceImpl{
		projectDao:       projectDao,
		financeDocDao:    financeDocDao,
		paymentConfigDao: paymentConfigDao,
	}
}

func TestCreateFinanceDocSuccess(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao)

	projectDao.On("GetByID", mock.Anything, int64(1)).Return(&model.Project{ID: 1, Name: "P1"}, nil).Once()
	paymentConfigDao.On("FindExistingPayAddresses", mock.Anything, []string{"0xabc123wallet001"}).
		Return(map[string]struct{}{"0xabc123wallet001": {}}, nil).Once()
	financeDocDao.On("Create", mock.Anything, mock.AnythingOfType("*model.FinanceDoc")).Return(nil).Once()

	docID, err := svc.CreateFinanceDoc(context.Background(), &data.CreateFinanceDocRequest{
		ProjectID: 1,
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100", Unit: "USD"},
		},
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, docID)
}

func TestCreateFinanceDocInvalidPayAddress(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao)

	projectDao.On("GetByID", mock.Anything, int64(1)).Return(&model.Project{ID: 1}, nil).Once()
	paymentConfigDao.On("FindExistingPayAddresses", mock.Anything, []string{"invalid"}).
		Return(map[string]struct{}{}, nil).Once()

	_, err := svc.CreateFinanceDoc(context.Background(), &data.CreateFinanceDocRequest{
		ProjectID: 1,
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "invalid", Amount: "100", Unit: "USD"},
		},
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidPayAddress, appErr.Code)
}

func TestUpdateFinanceDocSubmit(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao)

	doc := &model.FinanceDoc{
		DocID:     "doc-1",
		ProjectID: 1,
		Status:    model.FinanceDocStatusDraft,
	}
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	financeDocDao.On("UpdateStatus", mock.Anything, "doc-1", model.FinanceDocStatusToApprove, "submit").Return(nil).Once()

	resp, err := svc.UpdateFinanceDocStatus(context.Background(), "doc-1", &data.UpdateFinanceDocRequest{
		Status: model.FinanceDocStatusToApprove,
		Remark: "submit",
	})
	assert.NoError(t, err)
	assert.Equal(t, model.FinanceDocStatusToApprove, resp.Status)
}

func TestUpdateFinanceDocApprove(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao)

	doc := &model.FinanceDoc{
		DocID:  "doc-1",
		Status: model.FinanceDocStatusToApprove,
	}
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	financeDocDao.On("UpdateStatus", mock.Anything, "doc-1", model.FinanceDocStatusApproved, "ok").Return(nil).Once()

	resp, err := svc.UpdateFinanceDocStatus(context.Background(), "doc-1", &data.UpdateFinanceDocRequest{
		Status: model.FinanceDocStatusApproved,
		Remark: "ok",
	})
	assert.NoError(t, err)
	assert.Equal(t, model.FinanceDocStatusApproved, resp.Status)
}

func TestUpdateFinanceDocSubmitFromRejected(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao)

	doc := &model.FinanceDoc{
		DocID:  "doc-1",
		Status: model.FinanceDocStatusRejected,
	}
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	financeDocDao.On("UpdateStatus", mock.Anything, "doc-1", model.FinanceDocStatusToApprove, "resubmit").Return(nil).Once()

	resp, err := svc.UpdateFinanceDocStatus(context.Background(), "doc-1", &data.UpdateFinanceDocRequest{
		Status: model.FinanceDocStatusToApprove,
		Remark: "resubmit",
	})
	assert.NoError(t, err)
	assert.Equal(t, model.FinanceDocStatusToApprove, resp.Status)
}

func TestUpdateFinanceDocReject(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao)

	doc := &model.FinanceDoc{
		DocID:  "doc-1",
		Status: model.FinanceDocStatusToApprove,
	}
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	financeDocDao.On("UpdateStatus", mock.Anything, "doc-1", model.FinanceDocStatusRejected, "reject").Return(nil).Once()

	resp, err := svc.UpdateFinanceDocStatus(context.Background(), "doc-1", &data.UpdateFinanceDocRequest{
		Status: model.FinanceDocStatusRejected,
		Remark: "reject",
	})
	assert.NoError(t, err)
	assert.Equal(t, model.FinanceDocStatusRejected, resp.Status)
}

func TestUpdateFinanceDocInvalidTransition(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao)

	doc := &model.FinanceDoc{
		DocID:  "doc-1",
		Status: model.FinanceDocStatusDraft,
	}
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()

	_, err := svc.UpdateFinanceDocStatus(context.Background(), "doc-1", &data.UpdateFinanceDocRequest{
		Status: model.FinanceDocStatusApproved,
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidStatusTransition, appErr.Code)
}

func TestGetFinanceDocDetail(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao)

	detail, _ := json.Marshal([]data.ApplicationDetailItemVO{
		{PayAddress: "0xabc123wallet001", Amount: "100", Unit: "USD"},
	})
	doc := &model.FinanceDoc{
		DocID:             "doc-1",
		ProjectID:         1,
		Status:            model.FinanceDocStatusDraft,
		ApplicationDetail: detail,
	}
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	projectDao.On("GetByID", mock.Anything, int64(1)).Return(&model.Project{ID: 1, Name: "P1"}, nil).Once()

	vo, err := svc.GetFinanceDocDetail(context.Background(), "doc-1")
	assert.NoError(t, err)
	assert.Equal(t, "doc-1", vo.DocID)
	assert.NotNil(t, vo.Project)
	assert.Equal(t, "P1", vo.Project.Name)
}

func TestListPaymentConfigs(t *testing.T) {
	initServiceTestEnv()
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := &PaymentConfigServiceImpl{paymentConfigDao: paymentConfigDao}

	paymentConfigDao.On("ListAll", mock.Anything).Return([]*model.PaymentConfig{
		{PayAddress: "0xabc123wallet001", VoucherType: "crypto", PaymentAccount: "ACC-001"},
	}, nil).Once()

	items, err := svc.ListPaymentConfigs(context.Background())
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "0xabc123wallet001", items[0].PayAddress)
}
