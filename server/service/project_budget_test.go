package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao/mocks"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInitFromApprovedDocSuccess(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := &ProjectBudgetServiceImpl{
		financeDocDao:    financeDocDao,
		paymentConfigDao: paymentConfigDao,
		projectBudgetDao: projectBudgetDao,
		txBeginner:       passthroughTxBeginner{},
	}

	detail, _ := json.Marshal([]model.ApplicationDetailItem{
		{PayAddress: "0xabc123wallet001", Amount: "100"},
	})
	doc := &model.FinanceDoc{
		DocID:             "doc-1",
		ProjectID:         1,
		Status:            model.FinanceDocStatusApproved,
		ApplicationDetail: detail,
	}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xabc123wallet001").
		Return(&model.PaymentConfig{PayAddress: "0xabc123wallet001", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()
	projectBudgetDao.On("BatchCreate", mock.Anything, mock.Anything, []*model.ProjectBudget{
		{
			FinanceDocID:    "doc-1",
			ProjectID:       1,
			VoucherType:     util.VoucherTypeCrypto,
			Unit:            util.UnitCryptoUSDT,
			TotalAmount:     "0",
			AvailableAmount: "0",
			WithholdAmount:  "0",
			IssuedAmount:    "0",
			RefundAmount:    "0",
		},
	}).Return(nil).Once()

	err := svc.InitFromApprovedDoc(context.Background(), "doc-1")
	assert.NoError(t, err)
}

func TestInitFromApprovedDocIdempotent(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := &ProjectBudgetServiceImpl{
		financeDocDao:    financeDocDao,
		paymentConfigDao: paymentConfigDao,
		projectBudgetDao: projectBudgetDao,
		txBeginner:       passthroughTxBeginner{},
	}

	detail, _ := json.Marshal([]model.ApplicationDetailItem{
		{PayAddress: "0xabc123wallet001", Amount: "100"},
	})
	doc := &model.FinanceDoc{
		DocID:             "doc-1",
		ProjectID:         1,
		Status:            model.FinanceDocStatusApproved,
		ApplicationDetail: detail,
	}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xabc123wallet001").
		Return(&model.PaymentConfig{PayAddress: "0xabc123wallet001", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()
	projectBudgetDao.On("BatchCreate", mock.Anything, mock.Anything, mock.Anything).
		Return(dao.ErrBudgetAlreadyExists).Once()

	err := svc.InitFromApprovedDoc(context.Background(), "doc-1")
	assert.NoError(t, err)
}

func TestInitFromApprovedDocEmptyDocID(t *testing.T) {
	initServiceTestEnv()
	svc := &ProjectBudgetServiceImpl{txBeginner: passthroughTxBeginner{}}

	err := svc.InitFromApprovedDoc(context.Background(), " ")
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestInitFromApprovedDocNotFound(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	svc := &ProjectBudgetServiceImpl{
		financeDocDao: financeDocDao,
		txBeginner:    passthroughTxBeginner{},
	}

	financeDocDao.On("GetByDocID", mock.Anything, "missing").Return(nil, nil).Once()

	err := svc.InitFromApprovedDoc(context.Background(), "missing")
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeFinanceDocNotFound, appErr.Code)
}

func TestInitFromApprovedDocNotApproved(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	svc := &ProjectBudgetServiceImpl{
		financeDocDao: financeDocDao,
		txBeginner:    passthroughTxBeginner{},
	}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", Status: model.FinanceDocStatusDraft}, nil).Once()

	err := svc.InitFromApprovedDoc(context.Background(), "doc-1")
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidStatusTransition, appErr.Code)
}

func TestInitFromApprovedDocInvalidPayAddress(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
	svc := &ProjectBudgetServiceImpl{
		financeDocDao:    financeDocDao,
		paymentConfigDao: paymentConfigDao,
		txBeginner:       passthroughTxBeginner{},
	}

	detail, _ := json.Marshal([]model.ApplicationDetailItem{
		{PayAddress: "invalid", Amount: "100"},
	})
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{
			DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved, ApplicationDetail: detail,
		}, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "invalid").Return(nil, nil).Once()

	err := svc.InitFromApprovedDoc(context.Background(), "doc-1")
	assert.Error(t, err)
}

func TestInitFromApprovedDocBatchCreateError(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	paymentConfigDao := new(mocks.PaymentConfigDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := &ProjectBudgetServiceImpl{
		financeDocDao:    financeDocDao,
		paymentConfigDao: paymentConfigDao,
		projectBudgetDao: projectBudgetDao,
		txBeginner:       passthroughTxBeginner{},
	}

	detail, _ := json.Marshal([]model.ApplicationDetailItem{
		{PayAddress: "0xabc123wallet001", Amount: "100"},
	})
	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{
			DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved, ApplicationDetail: detail,
		}, nil).Once()
	paymentConfigDao.On("GetByPayAddress", mock.Anything, "0xabc123wallet001").
		Return(&model.PaymentConfig{PayAddress: "0xabc123wallet001", VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT}, nil).Once()
	projectBudgetDao.On("BatchCreate", mock.Anything, mock.Anything, mock.Anything).
		Return(assert.AnError).Once()

	err := svc.InitFromApprovedDoc(context.Background(), "doc-1")
	assert.Error(t, err)
}

func TestInitFromApprovedDocLoadDocError(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	svc := &ProjectBudgetServiceImpl{
		financeDocDao: financeDocDao,
		txBeginner:    passthroughTxBeginner{},
	}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(nil, assert.AnError).Once()

	err := svc.InitFromApprovedDoc(context.Background(), "doc-1")
	assert.Error(t, err)
}

func TestListByFinanceDocIDSuccess(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := &ProjectBudgetServiceImpl{
		financeDocDao:    financeDocDao,
		projectBudgetDao: projectBudgetDao,
	}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1}, nil).Once()
	projectBudgetDao.On("ListByFinanceDocID", mock.Anything, "doc-1").Return([]*model.ProjectBudget{
		{
			VoucherType:     util.VoucherTypeCrypto,
			Unit:            util.UnitCryptoUSDT,
			AvailableAmount: "80",
			TotalAmount:     "100",
			IssuedAmount:    "20",
		},
	}, nil).Once()

	items, err := svc.ListByFinanceDocID(context.Background(), "doc-1")
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, util.VoucherTypeCrypto, items[0].VoucherType)
	assert.Equal(t, "80", items[0].AvailableAmount)
	assert.Equal(t, "100", items[0].TotalAmount)
	assert.Equal(t, "20", items[0].IssuedAmount)
}

func TestListByFinanceDocIDEmptyDocID(t *testing.T) {
	initServiceTestEnv()
	svc := &ProjectBudgetServiceImpl{}

	_, err := svc.ListByFinanceDocID(context.Background(), " ")
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestListByFinanceDocIDNotFound(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	svc := &ProjectBudgetServiceImpl{financeDocDao: financeDocDao}

	financeDocDao.On("GetByDocID", mock.Anything, "missing").Return(nil, nil).Once()

	_, err := svc.ListByFinanceDocID(context.Background(), "missing")
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeFinanceDocNotFound, appErr.Code)
}

func TestListByFinanceDocIDLoadDocError(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	svc := &ProjectBudgetServiceImpl{financeDocDao: financeDocDao}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(nil, assert.AnError).Once()

	_, err := svc.ListByFinanceDocID(context.Background(), "doc-1")
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInternalError, appErr.Code)
}

func TestListByFinanceDocIDListError(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mocks.FinanceDocDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := &ProjectBudgetServiceImpl{
		financeDocDao:    financeDocDao,
		projectBudgetDao: projectBudgetDao,
	}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1"}, nil).Once()
	projectBudgetDao.On("ListByFinanceDocID", mock.Anything, "doc-1").
		Return(nil, assert.AnError).Once()

	_, err := svc.ListByFinanceDocID(context.Background(), "doc-1")
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInternalError, appErr.Code)
}

func TestListIssueBudgetByIssueRequestIDSuccess(t *testing.T) {
	initServiceTestEnv()
	issueRequestDao := new(mocks.IssueRequestDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
	svc := &ProjectBudgetServiceImpl{
		issueRequestDao: issueRequestDao,
		issueBudgetDao:  issueBudgetDao,
	}

	issueRequestDao.On("GetByID", mock.Anything, int64(11)).
		Return(&model.IssueRequest{ID: 11, ProjectID: 1}, nil).Once()
	issueBudgetDao.On("GetByIssueRequestID", mock.Anything, int64(11)).Return(&model.IssueBudget{
		VoucherType:     util.VoucherTypeCrypto,
		Unit:            util.UnitCryptoUSDT,
		AvailableAmount: "40",
		TotalAmount:     "50",
		IssuedAmount:    "10",
	}, nil).Once()

	items, err := svc.ListIssueBudgetByIssueRequestID(context.Background(), 11)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "40", items[0].AvailableAmount)
	assert.Equal(t, "50", items[0].TotalAmount)
}

func TestListIssueBudgetByIssueRequestIDEmptyWhenNoBudget(t *testing.T) {
	initServiceTestEnv()
	issueRequestDao := new(mocks.IssueRequestDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
	svc := &ProjectBudgetServiceImpl{
		issueRequestDao: issueRequestDao,
		issueBudgetDao:  issueBudgetDao,
	}

	issueRequestDao.On("GetByID", mock.Anything, int64(11)).
		Return(&model.IssueRequest{ID: 11}, nil).Once()
	issueBudgetDao.On("GetByIssueRequestID", mock.Anything, int64(11)).
		Return(nil, nil).Once()

	items, err := svc.ListIssueBudgetByIssueRequestID(context.Background(), 11)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestListIssueBudgetByIssueRequestIDInvalidID(t *testing.T) {
	initServiceTestEnv()
	svc := &ProjectBudgetServiceImpl{}

	_, err := svc.ListIssueBudgetByIssueRequestID(context.Background(), 0)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestListIssueBudgetByIssueRequestIDNotFound(t *testing.T) {
	initServiceTestEnv()
	issueRequestDao := new(mocks.IssueRequestDao)
	svc := &ProjectBudgetServiceImpl{issueRequestDao: issueRequestDao}

	issueRequestDao.On("GetByID", mock.Anything, int64(99)).Return(nil, nil).Once()

	_, err := svc.ListIssueBudgetByIssueRequestID(context.Background(), 99)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeIssueRequestNotFound, appErr.Code)
}

func TestListIssueBudgetByIssueRequestIDLoadErrors(t *testing.T) {
	initServiceTestEnv()
	issueRequestDao := new(mocks.IssueRequestDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
	svc := &ProjectBudgetServiceImpl{
		issueRequestDao: issueRequestDao,
		issueBudgetDao:  issueBudgetDao,
	}

	issueRequestDao.On("GetByID", mock.Anything, int64(11)).Return(nil, assert.AnError).Once()
	_, err := svc.ListIssueBudgetByIssueRequestID(context.Background(), 11)
	assert.Error(t, err)

	issueRequestDao.On("GetByID", mock.Anything, int64(12)).
		Return(&model.IssueRequest{ID: 12}, nil).Once()
	issueBudgetDao.On("GetByIssueRequestID", mock.Anything, int64(12)).
		Return(nil, assert.AnError).Once()
	_, err = svc.ListIssueBudgetByIssueRequestID(context.Background(), 12)
	assert.Error(t, err)
}
