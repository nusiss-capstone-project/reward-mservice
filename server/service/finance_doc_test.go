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

type mockFinanceDocApprovedProducer struct {
	mock.Mock
}

func (m *mockFinanceDocApprovedProducer) PublishFinanceDocApproved(ctx context.Context, docID string, projectID int64) error {
	args := m.Called(ctx, docID, projectID)
	return args.Error(0)
}

func newFinanceDocService(
	projectDao *mocks.ProjectDao,
	financeDocDao *mocks.FinanceDocDao,
	paymentConfigDao *mocks.PaymentConfigDao,
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	svc := newFinanceDocService(new(mocks.ProjectDao), new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

	_, err := svc.ListFinanceDocs(context.Background(), 0, 20)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidPagination, appErr.Code)
}

func TestGetFinanceDocDetailNotFound(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, new(mocks.PaymentConfigDao), nil)

	financeDocDao.On("GetByDocID", mock.Anything, "missing").Return(nil, nil).Once()

	_, err := svc.GetFinanceDocDetail(context.Background(), "missing")
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeFinanceDocNotFound, appErr.Code)
}

func TestApproveFinanceDocInvalidTransition(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, new(mocks.PaymentConfigDao), nil)

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
	projectDao := new(mocks.ProjectDao)
	svc := newFinanceDocService(projectDao, new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

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
	svc := newFinanceDocService(new(mocks.ProjectDao), new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

	_, err := svc.CreateFinanceDoc(context.Background(), nil)
	assert.Error(t, err)
}

func TestGetFinanceDocDetailEmptyDocID(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mocks.ProjectDao), new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

	_, err := svc.GetFinanceDocDetail(context.Background(), " ")
	assert.Error(t, err)
}

func TestUpdateFinanceDocStatusEmptyDocID(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mocks.ProjectDao), new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

	_, err := svc.UpdateFinanceDocStatus(context.Background(), " ", &data.UpdateFinanceDocRequest{
		Status: model.FinanceDocStatusToApprove,
	})
	assert.Error(t, err)
}

func TestApproveFinanceDocPublishFailure(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	approvedProducer := new(mockFinanceDocApprovedProducer)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, new(mocks.PaymentConfigDao), approvedProducer)

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
	svc := newFinanceDocService(new(mocks.ProjectDao), new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

	_, err := svc.CreateFinanceDoc(context.Background(), &data.CreateFinanceDocRequest{ProjectID: 1})
	assert.Error(t, err)
}

func TestUpdateFinanceDocStatusNilRequest(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mocks.ProjectDao), new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

	_, err := svc.UpdateFinanceDocStatus(context.Background(), "doc-1", nil)
	assert.Error(t, err)
}

func TestListFinanceDocsProjectNotFound(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	svc := newFinanceDocService(projectDao, financeDocDao, new(mocks.PaymentConfigDao), nil)

	financeDocDao.On("List", mock.Anything, 1, 20).Return([]*model.FinanceDoc{
		{DocID: "doc-1", ProjectID: 99, Status: model.FinanceDocStatusDraft},
	}, int64(1), nil).Once()
	projectDao.On("GetByID", mock.Anything, int64(99)).Return(nil, nil).Once()

	_, err := svc.ListFinanceDocs(context.Background(), 1, 20)
	assert.Error(t, err)
}

func TestCreateFinanceDocInvalidProjectID(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mocks.ProjectDao), new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

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
	svc := newFinanceDocService(new(mocks.ProjectDao), new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

	_, err := svc.ApproveFinanceDoc(context.Background(), "doc-1", nil)
	assert.Error(t, err)
}

func TestGetFinanceDocDetailLoadError(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, new(mocks.PaymentConfigDao), nil)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(nil, assert.AnError).Once()

	_, err := svc.GetFinanceDocDetail(context.Background(), "doc-1")
	assert.Error(t, err)
}

func TestCreateFinanceDocCreateFailed(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
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
	financeDocDao := new(mocks.FinanceDocDao)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, new(mocks.PaymentConfigDao), nil)

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
	financeDocDao := new(mocks.FinanceDocDao)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, new(mocks.PaymentConfigDao), nil)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(nil, assert.AnError).Once()

	_, err := svc.UpdateFinanceDocStatus(context.Background(), "doc-1", &data.UpdateFinanceDocRequest{
		Status: model.FinanceDocStatusToApprove,
	})
	assert.Error(t, err)
}

func TestUpdateFinanceDocSuccessFromDraft(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

	detail, _ := json.Marshal([]model.ApplicationDetailItem{
		{PayAddress: "0xabc123wallet001", Amount: "100"},
	})
	doc := &model.FinanceDoc{
		DocID:             "doc-1",
		ProjectID:         1,
		Status:            model.FinanceDocStatusDraft,
		ApplicationDetail: detail,
	}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xabc123wallet001").
		Return(&model.PaymentConfig{PayAddress: "0xabc123wallet001", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()
	financeDocDao.On("UpdateContent", mock.Anything, "doc-1", "updated desc", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	projectDao.On("GetByID", mock.Anything, int64(1)).Return(&model.Project{ID: 1, Name: "P1"}, nil).Once()

	vo, err := svc.UpdateFinanceDoc(context.Background(), "doc-1", &data.UpdateFinanceDocContentRequest{
		Description: "updated desc",
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "200"},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "doc-1", vo.DocID)
	assert.Equal(t, "updated desc", vo.Description)
	assert.Equal(t, "200", vo.ApplicationDetail[0].Amount)
}

func TestUpdateFinanceDocSuccessFromRejected(t *testing.T) {
	initServiceTestEnv()
	projectDao := new(mocks.ProjectDao)
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
	svc := newFinanceDocService(projectDao, financeDocDao, paymentConfigDao, nil)

	doc := &model.FinanceDoc{
		DocID:     "doc-1",
		ProjectID: 1,
		Status:    model.FinanceDocStatusRejected,
	}
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xabc123wallet001").
		Return(&model.PaymentConfig{PayAddress: "0xabc123wallet001", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()
	financeDocDao.On("UpdateContent", mock.Anything, "doc-1", "", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	projectDao.On("GetByID", mock.Anything, int64(1)).Return(&model.Project{ID: 1, Name: "P1"}, nil).Once()

	vo, err := svc.UpdateFinanceDoc(context.Background(), "doc-1", &data.UpdateFinanceDocContentRequest{
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, model.FinanceDocStatusRejected, vo.Status)
}

func TestUpdateFinanceDocNotEditable(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, new(mocks.PaymentConfigDao), nil)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", Status: model.FinanceDocStatusToApprove}, nil).Once()

	_, err := svc.UpdateFinanceDoc(context.Background(), "doc-1", &data.UpdateFinanceDocContentRequest{
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
		},
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidStatusTransition, appErr.Code)
}

func TestUpdateFinanceDocNotFound(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, new(mocks.PaymentConfigDao), nil)

	financeDocDao.On("GetByDocID", mock.Anything, "missing").Return(nil, nil).Once()

	_, err := svc.UpdateFinanceDoc(context.Background(), "missing", &data.UpdateFinanceDocContentRequest{
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
		},
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeFinanceDocNotFound, appErr.Code)
}

func TestUpdateFinanceDocEmptyDocID(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mocks.ProjectDao), new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

	_, err := svc.UpdateFinanceDoc(context.Background(), " ", &data.UpdateFinanceDocContentRequest{
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
		},
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestUpdateFinanceDocNilRequest(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mocks.ProjectDao), new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

	_, err := svc.UpdateFinanceDoc(context.Background(), "doc-1", nil)
	assert.Error(t, err)
}

func TestUpdateFinanceDocEmptyApplicationDetail(t *testing.T) {
	initServiceTestEnv()
	svc := newFinanceDocService(new(mocks.ProjectDao), new(mocks.FinanceDocDao), new(mocks.PaymentConfigDao), nil)

	_, err := svc.UpdateFinanceDoc(context.Background(), "doc-1", &data.UpdateFinanceDocContentRequest{})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestUpdateFinanceDocGetByDocIDFailed(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, new(mocks.PaymentConfigDao), nil)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(nil, assert.AnError).Once()

	_, err := svc.UpdateFinanceDoc(context.Background(), "doc-1", &data.UpdateFinanceDocContentRequest{
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
		},
	})
	assert.Error(t, err)
}

func TestUpdateFinanceDocInvalidPayAddress(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, paymentConfigDao, nil)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", Status: model.FinanceDocStatusDraft}, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "invalid").Return(nil, nil).Once()

	_, err := svc.UpdateFinanceDoc(context.Background(), "doc-1", &data.UpdateFinanceDocContentRequest{
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "invalid", Amount: "100"},
		},
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidPayAddress, appErr.Code)
}

func TestUpdateFinanceDocUpdateContentFailed(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, paymentConfigDao, nil)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", Status: model.FinanceDocStatusDraft}, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xabc123wallet001").
		Return(&model.PaymentConfig{PayAddress: "0xabc123wallet001", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()
	financeDocDao.On("UpdateContent", mock.Anything, "doc-1", "", mock.AnythingOfType("[]uint8")).
		Return(assert.AnError).Once()

	_, err := svc.UpdateFinanceDoc(context.Background(), "doc-1", &data.UpdateFinanceDocContentRequest{
		ApplicationDetail: []data.ApplicationDetailItemVO{
			{PayAddress: "0xabc123wallet001", Amount: "100"},
		},
	})
	assert.Error(t, err)
}

func TestUpdateFinanceDocDuplicateBudgetPair(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
	svc := newFinanceDocService(new(mocks.ProjectDao), financeDocDao, paymentConfigDao, nil)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", Status: model.FinanceDocStatusDraft}, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xabc123wallet001").
		Return(&model.PaymentConfig{PayAddress: "0xabc123wallet001", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xdef456wallet002").
		Return(&model.PaymentConfig{PayAddress: "0xdef456wallet002", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()

	_, err := svc.UpdateFinanceDoc(context.Background(), "doc-1", &data.UpdateFinanceDocContentRequest{
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
