package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type mockFinancePaymentDao struct {
	mock.Mock
}

func (m *mockFinancePaymentDao) Create(ctx context.Context, tx *gorm.DB, payment *model.FinancePayment) error {
	args := m.Called(ctx, tx, payment)
	return args.Error(0)
}

func (m *mockFinancePaymentDao) GetByPaymentIDAndDocID(ctx context.Context, paymentID, docID string) (*model.FinancePayment, error) {
	args := m.Called(ctx, paymentID, docID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.FinancePayment), args.Error(1)
}

func (m *mockFinancePaymentDao) ListByDocID(ctx context.Context, docID string) ([]*model.FinancePayment, error) {
	args := m.Called(ctx, docID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.FinancePayment), args.Error(1)
}

type passthroughTxBeginner struct{}

func (passthroughTxBeginner) Transaction(fc func(tx *gorm.DB) error, _ ...*sql.TxOptions) error {
	return fc(nil)
}

func newFinancePaymentService(
	financeDocDao *mockFinanceDocDao,
	paymentConfigDao *mockPaymentConfigDao,
	projectBudgetDao *mockProjectBudgetDao,
	financePaymentDao *mockFinancePaymentDao,
) *FinancePaymentServiceImpl {
	return &FinancePaymentServiceImpl{
		financeDocDao:     financeDocDao,
		paymentConfigDao:  paymentConfigDao,
		projectBudgetDao:  projectBudgetDao,
		financePaymentDao: financePaymentDao,
		txBeginner:        passthroughTxBeginner{},
	}
}

func TestCreateFinancePaymentSuccess(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	financePaymentDao := new(mockFinancePaymentDao)
	svc := newFinancePaymentService(financeDocDao, paymentConfigDao, projectBudgetDao, financePaymentDao)

	detail, _ := json.Marshal([]model.ApplicationDetailItem{
		{PayAddress: "0xabc123wallet001", Amount: "100", Unit: "USD"},
	})
	doc := &model.FinanceDoc{
		DocID:             "doc-1",
		Status:            model.FinanceDocStatusApproved,
		ApplicationDetail: detail,
	}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	paymentConfigDao.On("GetByPayAddressAndUnit", mock.Anything, "0xabc123wallet001", "USD").
		Return(&model.PaymentConfig{
			PayAddress:  "0xabc123wallet001",
			VoucherType: "crypto",
			Unit:        "USD",
		}, nil).Once()
	projectBudgetDao.On("GetByDocIDVoucherTypeUnit", mock.Anything, "doc-1", "crypto", "USD").
		Return(&model.ProjectBudget{ID: 1, TotalAmount: "0"}, nil).Once()
	projectBudgetDao.On("LockByID", mock.Anything, mock.Anything, int64(1)).
		Return(&model.ProjectBudget{ID: 1, TotalAmount: "0"}, nil).Once()
	financePaymentDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.FinancePayment")).Return(nil).Once()
	projectBudgetDao.On("UpdateTotalAndAvailableAmount", mock.Anything, mock.Anything, int64(1), "100").Return(nil).Once()

	result, err := svc.CreateFinancePayment(context.Background(), "doc-1", &data.CreateFinancePaymentRequest{
		PaymentAddress: "0xabc123wallet001",
		Amount:         "100",
		Unit:           "USD",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, result.PaymentID)
	assert.Equal(t, model.FinancePaymentStatusPaid, result.PaymentStatus)
	assert.Equal(t, "100", result.Amount)
}

func TestCreateFinancePaymentExceedsDocAmount(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	financePaymentDao := new(mockFinancePaymentDao)
	svc := newFinancePaymentService(financeDocDao, paymentConfigDao, projectBudgetDao, financePaymentDao)

	detail, _ := json.Marshal([]model.ApplicationDetailItem{
		{PayAddress: "0xabc123wallet001", Amount: "100", Unit: "USD"},
	})
	doc := &model.FinanceDoc{
		DocID:             "doc-1",
		Status:            model.FinanceDocStatusApproved,
		ApplicationDetail: detail,
	}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	paymentConfigDao.On("GetByPayAddressAndUnit", mock.Anything, "0xabc123wallet001", "USD").
		Return(&model.PaymentConfig{PayAddress: "0xabc123wallet001", VoucherType: "crypto", Unit: "USD"}, nil).Once()
	projectBudgetDao.On("GetByDocIDVoucherTypeUnit", mock.Anything, "doc-1", "crypto", "USD").
		Return(&model.ProjectBudget{TotalAmount: "60"}, nil).Once()

	_, err := svc.CreateFinancePayment(context.Background(), "doc-1", &data.CreateFinancePaymentRequest{
		PaymentAddress: "0xabc123wallet001",
		Amount:         "50",
		Unit:           "USD",
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodePaymentExceedsDocAmount, appErr.Code)
	financePaymentDao.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateFinancePaymentDocNotApproved(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	financePaymentDao := new(mockFinancePaymentDao)
	svc := newFinancePaymentService(financeDocDao, paymentConfigDao, projectBudgetDao, financePaymentDao)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", Status: model.FinanceDocStatusDraft}, nil).Once()

	_, err := svc.CreateFinancePayment(context.Background(), "doc-1", &data.CreateFinancePaymentRequest{
		PaymentAddress: "0xabc123wallet001",
		Amount:         "10",
		Unit:           "USD",
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeFinanceDocNotApproved, appErr.Code)
}

func TestGetFinancePaymentSuccess(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	financePaymentDao := new(mockFinancePaymentDao)
	svc := newFinancePaymentService(financeDocDao, paymentConfigDao, projectBudgetDao, financePaymentDao)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1"}, nil).Once()
	financePaymentDao.On("GetByPaymentIDAndDocID", mock.Anything, "pay-1", "doc-1").
		Return(&model.FinancePayment{
			FinanceDocID:   "doc-1",
			PaymentID:      "pay-1",
			PaymentAddress: "0xabc123wallet001",
			Amount:         "10",
			PaymentStatus:  model.FinancePaymentStatusPaid,
		}, nil).Once()

	result, err := svc.GetFinancePayment(context.Background(), "doc-1", "pay-1")
	assert.NoError(t, err)
	assert.Equal(t, "pay-1", result.PaymentID)
}

func TestGetFinancePaymentNotFound(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	financePaymentDao := new(mockFinancePaymentDao)
	svc := newFinancePaymentService(financeDocDao, paymentConfigDao, projectBudgetDao, financePaymentDao)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1"}, nil).Once()
	financePaymentDao.On("GetByPaymentIDAndDocID", mock.Anything, "pay-1", "doc-1").
		Return(nil, nil).Once()

	_, err := svc.GetFinancePayment(context.Background(), "doc-1", "pay-1")
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeFinancePaymentNotFound, appErr.Code)
}

func TestGetFinancePaymentListByDocIDSuccess(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	financePaymentDao := new(mockFinancePaymentDao)
	svc := newFinancePaymentService(financeDocDao, paymentConfigDao, projectBudgetDao, financePaymentDao)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1"}, nil).Once()
	financePaymentDao.On("ListByDocID", mock.Anything, "doc-1").
		Return([]*model.FinancePayment{
			{
				FinanceDocID:   "doc-1",
				PaymentID:      "pay-1",
				PaymentAddress: "0xabc123wallet001",
				Amount:         "10",
				PaymentStatus:  model.FinancePaymentStatusPaid,
			},
			{
				FinanceDocID:   "doc-1",
				PaymentID:      "pay-2",
				PaymentAddress: "0xabc123wallet001",
				Amount:         "20",
				PaymentStatus:  model.FinancePaymentStatusPaid,
			},
		}, nil).Once()

	result, err := svc.GetFinancePaymentListByDocID(context.Background(), "doc-1")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "pay-1", result[0].PaymentID)
	assert.Equal(t, "pay-2", result[1].PaymentID)
}

func TestGetFinancePaymentListByDocIDEmpty(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	financePaymentDao := new(mockFinancePaymentDao)
	svc := newFinancePaymentService(financeDocDao, paymentConfigDao, projectBudgetDao, financePaymentDao)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1"}, nil).Once()
	financePaymentDao.On("ListByDocID", mock.Anything, "doc-1").
		Return([]*model.FinancePayment{}, nil).Once()

	result, err := svc.GetFinancePaymentListByDocID(context.Background(), "doc-1")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetFinancePaymentListByDocIDDocNotFound(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	financePaymentDao := new(mockFinancePaymentDao)
	svc := newFinancePaymentService(financeDocDao, paymentConfigDao, projectBudgetDao, financePaymentDao)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(nil, nil).Once()

	_, err := svc.GetFinancePaymentListByDocID(context.Background(), "doc-1")
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeFinanceDocNotFound, appErr.Code)
}

var _ repository.TxBeginner = passthroughTxBeginner{}
