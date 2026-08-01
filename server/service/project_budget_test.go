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
