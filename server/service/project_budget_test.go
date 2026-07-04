package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type mockProjectBudgetDao struct {
	mock.Mock
}

func (m *mockProjectBudgetDao) BatchCreate(ctx context.Context, tx *gorm.DB, budgets []*model.ProjectBudget) error {
	args := m.Called(ctx, tx, budgets)
	return args.Error(0)
}

func (m *mockProjectBudgetDao) CountByFinanceDocID(ctx context.Context, docID string) (int64, error) {
	args := m.Called(ctx, docID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockProjectBudgetDao) GetByDocIDVoucherTypeUnit(ctx context.Context, docID, voucherType, unit string) (*model.ProjectBudget, error) {
	args := m.Called(ctx, docID, voucherType, unit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ProjectBudget), args.Error(1)
}

func (m *mockProjectBudgetDao) GetByProjectIDVoucherTypeUnit(
	ctx context.Context,
	projectID int64,
	voucherType, unit string,
) (*model.ProjectBudget, error) {
	args := m.Called(ctx, projectID, voucherType, unit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ProjectBudget), args.Error(1)
}

func (m *mockProjectBudgetDao) ApplySubmitWithhold(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error {
	args := m.Called(ctx, tx, budgetID, amount)
	return args.Error(0)
}

func (m *mockProjectBudgetDao) ApplyApproveIssued(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error {
	args := m.Called(ctx, tx, budgetID, amount)
	return args.Error(0)
}

func (m *mockProjectBudgetDao) ApplyRejectRelease(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error {
	args := m.Called(ctx, tx, budgetID, amount)
	return args.Error(0)
}

func (m *mockProjectBudgetDao) LockByID(ctx context.Context, tx *gorm.DB, budgetID int64) (*model.ProjectBudget, error) {
	args := m.Called(ctx, tx, budgetID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ProjectBudget), args.Error(1)
}

func (m *mockProjectBudgetDao) UpdateTotalAndAvailableAmount(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error {
	args := m.Called(ctx, tx, budgetID, amount)
	return args.Error(0)
}

func TestInitFromApprovedDocSuccess(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	svc := &ProjectBudgetServiceImpl{
		financeDocDao:    financeDocDao,
		paymentConfigDao: paymentConfigDao,
		projectBudgetDao: projectBudgetDao,
		txBeginner:       passthroughTxBeginner{},
	}

	detail, _ := json.Marshal([]model.ApplicationDetailItem{
		{PayAddress: "0xabc123wallet001", Amount: "100", Unit: "USD"},
	})
	doc := &model.FinanceDoc{
		DocID:             "doc-1",
		ProjectID:         1,
		Status:            model.FinanceDocStatusApproved,
		ApplicationDetail: detail,
	}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	paymentConfigDao.On("MapByPayAddresses", mock.Anything, []string{"0xabc123wallet001"}).
		Return(map[string]*model.PaymentConfig{
			"0xabc123wallet001": {PayAddress: "0xabc123wallet001", VoucherType: "crypto"},
		}, nil).Once()
	projectBudgetDao.On("BatchCreate", mock.Anything, mock.Anything, []*model.ProjectBudget{
		{
			FinanceDocID:    "doc-1",
			ProjectID:       1,
			VoucherType:     "crypto",
			Unit:            "USD",
			TotalAmount:     "0",
			AvailableAmount: "100",
			WitholdAmount:   "0",
			IssuedAmount:    "0",
			RefundAmount:    "0",
		},
	}).Return(nil).Once()

	err := svc.InitFromApprovedDoc(context.Background(), "doc-1")
	assert.NoError(t, err)
}

func TestInitFromApprovedDocIdempotent(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	paymentConfigDao := new(mockPaymentConfigDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	svc := &ProjectBudgetServiceImpl{
		financeDocDao:    financeDocDao,
		paymentConfigDao: paymentConfigDao,
		projectBudgetDao: projectBudgetDao,
		txBeginner:       passthroughTxBeginner{},
	}

	detail, _ := json.Marshal([]model.ApplicationDetailItem{
		{PayAddress: "0xabc123wallet001", Amount: "100", Unit: "USD"},
	})
	doc := &model.FinanceDoc{
		DocID:             "doc-1",
		ProjectID:         1,
		Status:            model.FinanceDocStatusApproved,
		ApplicationDetail: detail,
	}

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").Return(doc, nil).Once()
	paymentConfigDao.On("MapByPayAddresses", mock.Anything, []string{"0xabc123wallet001"}).
		Return(map[string]*model.PaymentConfig{
			"0xabc123wallet001": {PayAddress: "0xabc123wallet001", VoucherType: "crypto"},
		}, nil).Once()
	projectBudgetDao.On("BatchCreate", mock.Anything, mock.Anything, mock.Anything).
		Return(dao.ErrBudgetAlreadyExists).Once()
	projectBudgetDao.On("CountByFinanceDocID", mock.Anything, "doc-1").Return(int64(1), nil).Once()

	err := svc.InitFromApprovedDoc(context.Background(), "doc-1")
	assert.NoError(t, err)
}
