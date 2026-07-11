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

func (m *mockFinanceDocDao) ExistsByProjectID(ctx context.Context, projectID int64) (bool, error) {
	args := m.Called(ctx, projectID)
	return args.Bool(0), args.Error(1)
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

func (m *mockPaymentConfigDao) GetByPayAddress(ctx context.Context, payAddress string) (*model.PaymentConfig, error) {
	args := m.Called(ctx, payAddress)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PaymentConfig), args.Error(1)
}

type mockFinanceDocApprovedProducer struct {
	mock.Mock
}

func (m *mockFinanceDocApprovedProducer) PublishFinanceDocApproved(ctx context.Context, docID string, projectID int64) error {
	args := m.Called(ctx, docID, projectID)
	return args.Error(0)
}

func newFinanceDocService(
	projectDao *mockProjectDao,
	financeDocDao *mockFinanceDocDao,
	paymentConfigDao *mockPaymentConfigDao,
	approvedProducer *mockFinanceDocApprovedProducer,
) *FinanceDocServiceImpl {
	if approvedProducer == nil {
		approvedProducer = new(mockFinanceDocApprovedProducer)
	}
	return &FinanceDocServiceImpl{
		projectDao:                 projectDao,
		financeDocDao:              financeDocDao,
		paymentConfigDao:           paymentConfigDao,
		financeDocApprovedProducer: approvedProducer,
	}
}

func TestCreateFinanceDocSuccess(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

	projectDao.On("GetByID", mock.Anything, int64(1)).Return(&model.Project{ID: 1, Name: "P1"}, nil).Once()
	financeDocDao.On("ExistsByProjectID", mock.Anything, int64(1)).Return(false, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xabc123wallet001").
		Return(&model.PaymentConfig{PayAddress: "0xabc123wallet001", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()
	financeDocDao.On("Create", mock.Anything, mock.AnythingOfType("*model.FinanceDoc")).Return(nil).Once()

	docID, err := svc.CreateFinanceDoc(context.Background(), &data.CreateFinanceDocRequest{
		ProjectID: 1,
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
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
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

	projectDao.On("GetByID", mock.Anything, int64(1)).Return(&model.Project{ID: 1}, nil).Once()
	financeDocDao.On("ExistsByProjectID", mock.Anything, int64(1)).Return(false, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "invalid").Return(nil, nil).Once()

	_, err := svc.CreateFinanceDoc(context.Background(), &data.CreateFinanceDocRequest{
		ProjectID: 1,
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "invalid", Amount: "100"},
		},
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidPayAddress, appErr.Code)
}

func TestCreateFinanceDocProjectAlreadyExists(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

	projectDao.On("GetByID", mock.Anything, int64(1)).Return(&model.Project{ID: 1}, nil).Once()
	financeDocDao.On("ExistsByProjectID", mock.Anything, int64(1)).Return(true, nil).Once()

	_, err := svc.CreateFinanceDoc(context.Background(), &data.CreateFinanceDocRequest{
		ProjectID: 1,
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
		},
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeFinanceDocProjectExists, appErr.Code)
}

func TestCreateFinanceDocDuplicateBudgetPair(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

	projectDao.On("GetByID", mock.Anything, int64(1)).Return(&model.Project{ID: 1}, nil).Once()
	financeDocDao.On("ExistsByProjectID", mock.Anything, int64(1)).Return(false, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xabc123wallet001").
		Return(&model.PaymentConfig{PayAddress: "0xabc123wallet001", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xdef456wallet002").
		Return(&model.PaymentConfig{PayAddress: "0xdef456wallet002", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()

	_, err := svc.CreateFinanceDoc(context.Background(), &data.CreateFinanceDocRequest{
		ProjectID: 1,
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
			{PayAddress: "0xdef456wallet002", Amount: "200"},
		},
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeDuplicateBudgetPair, appErr.Code)
}

func TestUpdateFinanceDocSubmit(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

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

func TestApproveFinanceDocApprovedPublishesEvent(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	approvedProducer := new(mockFinanceDocApprovedProducer)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, approvedProducer)

	doc := &model.FinanceDoc{
		DocID:     "doc-1",
		ProjectID: 1,
		Status:    model.FinanceDocStatusToApprove,
	}
	approvedDoc := &model.FinanceDoc{
		DocID:     "doc-1",
		ProjectID: 1,
		Status:    model.FinanceDocStatusApproved,
	}
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	financeDocDao.On("UpdateStatus", mock.Anything, "doc-1", model.FinanceDocStatusApproved, "ok").Return(nil).Once()
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(approvedDoc, nil).Once()
	approvedProducer.On("PublishFinanceDocApproved", mock.Anything, "doc-1", int64(1)).Return(nil).Once()

	resp, err := svc.ApproveFinanceDoc(context.Background(), "doc-1", &data.ApproveFinanceDocRequest{
		Status: model.FinanceDocStatusApproved,
		Remark: "ok",
	})
	assert.NoError(t, err)
	assert.Equal(t, model.FinanceDocStatusApproved, resp.Status)
}

func TestApproveFinanceDocRejectedDoesNotPublish(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	approvedProducer := new(mockFinanceDocApprovedProducer)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, approvedProducer)

	doc := &model.FinanceDoc{
		DocID:  "doc-1",
		Status: model.FinanceDocStatusToApprove,
	}
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	financeDocDao.On("UpdateStatus", mock.Anything, "doc-1", model.FinanceDocStatusRejected, "reject").Return(nil).Once()

	resp, err := svc.ApproveFinanceDoc(context.Background(), "doc-1", &data.ApproveFinanceDocRequest{
		Status: model.FinanceDocStatusRejected,
		Remark: "reject",
	})
	assert.NoError(t, err)
	assert.Equal(t, model.FinanceDocStatusRejected, resp.Status)
	approvedProducer.AssertNotCalled(t, "PublishFinanceDocApproved", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateFinanceDocSubmitFromRejected(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

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
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

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
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

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
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

	detail, _ := json.Marshal([]data.ApplicationDetailItemVO{
		{PayAddress: "0xabc123wallet001", Amount: "100"},
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
		{PayAddress: "0xabc123wallet001", VoucherType: util.VoucherTypeCrypto, PaymentAccount: "ACC-001"},
	}, nil).Once()

	items, err := svc.ListPaymentConfigs(context.Background())
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "0xabc123wallet001", items[0].PayAddress)
}

func TestListFinanceDocs(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

	detail, _ := json.Marshal([]data.ApplicationDetailItemVO{
		{PayAddress: "0xabc123wallet001", Amount: "100"},
	})
	financeDocDao.On("List", mock.Anything, 1, 20).Return([]*model.FinanceDoc{
		{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusDraft, ApplicationDetail: detail},
	}, int64(1), nil).Once()
	projectDao.On("GetByID", mock.Anything, int64(1)).Return(&model.Project{ID: 1, Name: "P1"}, nil).Once()

	result, err := svc.ListFinanceDocs(context.Background(), 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	items := result.Items.([]*data.FinanceDocVO)
	assert.Len(t, items, 1)
	assert.Equal(t, "doc-1", items[0].DocID)
}

func TestListFinanceDocsInvalidPagination(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mockProjectDao), new(mockFinanceDocDao), new(mockPaymentConfigDao), nil)

	_, err := svc.ListFinanceDocs(context.Background(), 0, 20)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidPagination, appErr.Code)
}

func TestGetFinanceDocDetailNotFound(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	svc := newFinanceDocService(new(mockProjectDao), financeDocDao, new(mockPaymentConfigDao), nil)

	financeDocDao.On("GetByDocID", mock.Anything, "missing").Return(nil, nil).Once()

	_, err := svc.GetFinanceDocDetail(context.Background(), "missing")
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeFinanceDocNotFound, appErr.Code)
}

func TestApproveFinanceDocInvalidTransition(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	svc := newFinanceDocService(new(mockProjectDao), financeDocDao, new(mockPaymentConfigDao), nil)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", Status: model.FinanceDocStatusDraft}, nil).Once()

	_, err := svc.ApproveFinanceDoc(context.Background(), "doc-1", &data.ApproveFinanceDocRequest{
		Status: model.FinanceDocStatusApproved,
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidStatusTransition, appErr.Code)
}

func TestCreateFinanceDocProjectNotFound(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	svc := newFinanceDocService(projectDao, new(mockFinanceDocDao), new(mockPaymentConfigDao), nil)

	projectDao.On("GetByID", mock.Anything, int64(99)).Return(nil, nil).Once()

	_, err := svc.CreateFinanceDoc(context.Background(), &data.CreateFinanceDocRequest{
		ProjectID: 99,
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
		},
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeProjectNotFound, appErr.Code)
}

func TestCreateFinanceDocNilRequest(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mockProjectDao), new(mockFinanceDocDao), new(mockPaymentConfigDao), nil)

	_, err := svc.CreateFinanceDoc(context.Background(), nil)
	assert.Error(t, err)
}

func TestGetFinanceDocDetailEmptyDocID(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mockProjectDao), new(mockFinanceDocDao), new(mockPaymentConfigDao), nil)

	_, err := svc.GetFinanceDocDetail(context.Background(), " ")
	assert.Error(t, err)
}

func TestUpdateFinanceDocStatusEmptyDocID(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mockProjectDao), new(mockFinanceDocDao), new(mockPaymentConfigDao), nil)

	_, err := svc.UpdateFinanceDocStatus(context.Background(), " ", &data.UpdateFinanceDocRequest{
		Status: model.FinanceDocStatusToApprove,
	})
	assert.Error(t, err)
}

func TestApproveFinanceDocPublishFailure(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	approvedProducer := new(mockFinanceDocApprovedProducer)
	svc := newFinanceDocService(new(mockProjectDao), financeDocDao, new(mockPaymentConfigDao), approvedProducer)

	doc := &model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusToApprove}
	approvedDoc := &model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	financeDocDao.On("UpdateStatus", mock.Anything, "doc-1", model.FinanceDocStatusApproved, "ok").Return(nil).Once()
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(approvedDoc, nil).Once()
	approvedProducer.On("PublishFinanceDocApproved", mock.Anything, "doc-1", int64(1)).
		Return(assert.AnError).Once()

	_, err := svc.ApproveFinanceDoc(context.Background(), "doc-1", &data.ApproveFinanceDocRequest{
		Status: model.FinanceDocStatusApproved,
		Remark: "ok",
	})
	assert.Error(t, err)
}

func TestCreateFinanceDocEmptyApplicationDetail(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mockProjectDao), new(mockFinanceDocDao), new(mockPaymentConfigDao), nil)

	_, err := svc.CreateFinanceDoc(context.Background(), &data.CreateFinanceDocRequest{ProjectID: 1})
	assert.Error(t, err)
}

func TestUpdateFinanceDocStatusNilRequest(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mockProjectDao), new(mockFinanceDocDao), new(mockPaymentConfigDao), nil)

	_, err := svc.UpdateFinanceDocStatus(context.Background(), "doc-1", nil)
	assert.Error(t, err)
}

func TestListFinanceDocsProjectNotFound(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	svc := newFinanceDocService(projectDao, financeDocDao, new(mockPaymentConfigDao), nil)

	financeDocDao.On("List", mock.Anything, 1, 20).Return([]*model.FinanceDoc{
		{DocID: "doc-1", ProjectID: 99, Status: model.FinanceDocStatusDraft},
	}, int64(1), nil).Once()
	projectDao.On("GetByID", mock.Anything, int64(99)).Return(nil, nil).Once()

	_, err := svc.ListFinanceDocs(context.Background(), 1, 20)
	assert.Error(t, err)
}

func TestCreateFinanceDocInvalidProjectID(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mockProjectDao), new(mockFinanceDocDao), new(mockPaymentConfigDao), nil)

	_, err := svc.CreateFinanceDoc(context.Background(), &data.CreateFinanceDocRequest{
		ProjectID: 0,
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
		},
	})
	assert.Error(t, err)
}

func TestApproveFinanceDocNilRequest(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mockProjectDao), new(mockFinanceDocDao), new(mockPaymentConfigDao), nil)

	_, err := svc.ApproveFinanceDoc(context.Background(), "doc-1", nil)
	assert.Error(t, err)
}

func TestGetFinanceDocDetailLoadError(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	svc := newFinanceDocService(new(mockProjectDao), financeDocDao, new(mockPaymentConfigDao), nil)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(nil, assert.AnError).Once()

	_, err := svc.GetFinanceDocDetail(context.Background(), "doc-1")
	assert.Error(t, err)
}

func TestCreateFinanceDocCreateFailed(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mockProjectDao)
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

	projectDao.On("GetByID", mock.Anything, int64(1)).Return(&model.Project{ID: 1}, nil).Once()
	financeDocDao.On("ExistsByProjectID", mock.Anything, int64(1)).Return(false, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xabc123wallet001").
		Return(&model.PaymentConfig{PayAddress: "0xabc123wallet001", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()
	financeDocDao.On("Create", mock.Anything, mock.AnythingOfType("*model.FinanceDoc")).Return(assert.AnError).Once()

	_, err := svc.CreateFinanceDoc(context.Background(), &data.CreateFinanceDocRequest{
		ProjectID: 1,
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
		},
	})
	assert.Error(t, err)
}

func TestUpdateFinanceDocStatusNotFound(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	svc := newFinanceDocService(new(mockProjectDao), financeDocDao, new(mockPaymentConfigDao), nil)

	financeDocDao.On("GetByDocID", mock.Anything, "missing").Return(nil, nil).Once()

	_, err := svc.UpdateFinanceDocStatus(context.Background(), "missing", &data.UpdateFinanceDocRequest{
		Status: model.FinanceDocStatusToApprove,
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeFinanceDocNotFound, appErr.Code)
}

func TestUpdateFinanceDocStatusLoadError(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	svc := newFinanceDocService(new(mockProjectDao), financeDocDao, new(mockPaymentConfigDao), nil)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(nil, assert.AnError).Once()

	_, err := svc.UpdateFinanceDocStatus(context.Background(), "doc-1", &data.UpdateFinanceDocRequest{
		Status: model.FinanceDocStatusToApprove,
	})
	assert.Error(t, err)
}
