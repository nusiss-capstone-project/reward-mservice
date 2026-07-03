package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockProjectBudgetDao struct {
	mock.Mock
}

func (m *mockProjectBudgetDao) CreateFromApprovedDoc(ctx context.Context, docID string, projectID int64, items []dao.BudgetCreateItem) error {
	args := m.Called(ctx, docID, projectID, items)
	return args.Error(0)
}

func (m *mockProjectBudgetDao) CountByFinanceDocID(ctx context.Context, docID string) (int64, error) {
	args := m.Called(ctx, docID)
	return args.Get(0).(int64), args.Error(1)
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
	projectBudgetDao.On("CreateFromApprovedDoc", mock.Anything, "doc-1", int64(1), []dao.BudgetCreateItem{
		{VoucherType: "crypto", Unit: "USD", Amount: "100"},
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
	projectBudgetDao.On("CreateFromApprovedDoc", mock.Anything, "doc-1", int64(1), mock.Anything).
		Return(dao.ErrBudgetAlreadyExists).Once()
	projectBudgetDao.On("CountByFinanceDocID", mock.Anything, "doc-1").Return(int64(1), nil).Once()

	err := svc.InitFromApprovedDoc(context.Background(), "doc-1")
	assert.NoError(t, err)
}
