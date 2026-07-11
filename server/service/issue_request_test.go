package service

import (
	"context"
	"testing"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type mockIssueRequestDao struct {
	mock.Mock
}

func (m *mockIssueRequestDao) Create(ctx context.Context, request *model.IssueRequest) error {
	args := m.Called(ctx, request)
	return args.Error(0)
}

func (m *mockIssueRequestDao) GetByID(ctx context.Context, id int64) (*model.IssueRequest, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.IssueRequest), args.Error(1)
}

func (m *mockIssueRequestDao) GetByIDForUpdate(ctx context.Context, tx *gorm.DB, id int64) (*model.IssueRequest, error) {
	args := m.Called(ctx, tx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.IssueRequest), args.Error(1)
}

func (m *mockIssueRequestDao) ListByProjectID(ctx context.Context, projectID int64, page, size int) ([]*model.IssueRequest, int64, error) {
	args := m.Called(ctx, projectID, page, size)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*model.IssueRequest), args.Get(1).(int64), args.Error(2)
}

func (m *mockIssueRequestDao) UpdateFields(ctx context.Context, id int64, voucherType, unit, amount, remark string) error {
	args := m.Called(ctx, id, voucherType, unit, amount, remark)
	return args.Error(0)
}

func (m *mockIssueRequestDao) UpdateStatusInTx(ctx context.Context, tx *gorm.DB, id int64, fromStatus, toStatus, remark string) error {
	args := m.Called(ctx, tx, id, fromStatus, toStatus, remark)
	return args.Error(0)
}

func (m *mockIssueRequestDao) MarkOngoing(ctx context.Context, tx *gorm.DB, id int64) error {
	args := m.Called(ctx, tx, id)
	return args.Error(0)
}

type mockIssueBudgetDao struct {
	mock.Mock
}

func (m *mockIssueBudgetDao) Create(ctx context.Context, tx *gorm.DB, budget *model.IssueBudget) error {
	args := m.Called(ctx, tx, budget)
	return args.Error(0)
}

func (m *mockIssueBudgetDao) GetByIssueRequestID(ctx context.Context, issueRequestID int64) (*model.IssueBudget, error) {
	args := m.Called(ctx, issueRequestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.IssueBudget), args.Error(1)
}

func (m *mockIssueBudgetDao) GetByID(ctx context.Context, id int64) (*model.IssueBudget, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.IssueBudget), args.Error(1)
}

func (m *mockIssueBudgetDao) GetFirstAvailableForDistribution(
	ctx context.Context,
	projectID int64,
	voucherType, unit, amount string,
) (*model.IssueBudget, error) {
	args := m.Called(ctx, projectID, voucherType, unit, amount)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.IssueBudget), args.Error(1)
}

func (m *mockIssueBudgetDao) LockByID(ctx context.Context, tx *gorm.DB, budgetID int64) (*model.IssueBudget, error) {
	args := m.Called(ctx, tx, budgetID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.IssueBudget), args.Error(1)
}

func (m *mockIssueBudgetDao) ApplyDistributionDeductAvailable(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error {
	args := m.Called(ctx, tx, budgetID, amount)
	return args.Error(0)
}

func (m *mockIssueBudgetDao) ApplyDistributionIssued(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error {
	args := m.Called(ctx, tx, budgetID, amount)
	return args.Error(0)
}

func (m *mockIssueBudgetDao) ApplyDistributionRefund(ctx context.Context, tx *gorm.DB, budgetID int64, amount string) error {
	args := m.Called(ctx, tx, budgetID, amount)
	return args.Error(0)
}

type mockIssueRequestUpdatedProducer struct {
	mock.Mock
}

func (m *mockIssueRequestUpdatedProducer) PublishIssueRequestUpdated(ctx context.Context, issueRequestID int64) error {
	args := m.Called(ctx, issueRequestID)
	return args.Error(0)
}

func newIssueRequestService(
	financeDocDao *mockFinanceDocDao,
	projectBudgetDao *mockProjectBudgetDao,
	issueRequestDao *mockIssueRequestDao,
	issueBudgetDao *mockIssueBudgetDao,
	producer *mockIssueRequestUpdatedProducer,
) *IssueRequestServiceImpl {
	return &IssueRequestServiceImpl{
		financeDocDao:    financeDocDao,
		projectBudgetDao: projectBudgetDao,
		issueRequestDao:  issueRequestDao,
		issueBudgetDao:   issueBudgetDao,
		eventProducer:    producer,
		txBeginner:       passthroughTxBeginner{},
	}
}

func TestCreateIssueRequestSuccess(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	producer := new(mockIssueRequestUpdatedProducer)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, issueBudgetDao, producer)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{AvailableAmount: "100"}, nil).Once()
	issueRequestDao.On("Create", mock.Anything, mock.AnythingOfType("*model.IssueRequest")).Return(nil).Once()

	result, err := svc.CreateIssueRequest(context.Background(), "doc-1", &data.CreateIssueRequestRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "100",
		ExpenseType: "REWARD",
	})
	assert.NoError(t, err)
	assert.Equal(t, model.IssueRequestStatusDraft, result.RequestStatus)
}

func TestCreateIssueRequestInsufficientAvailable(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	producer := new(mockIssueRequestUpdatedProducer)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, issueBudgetDao, producer)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{AvailableAmount: "50"}, nil).Once()

	_, err := svc.CreateIssueRequest(context.Background(), "doc-1", &data.CreateIssueRequestRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "100",
		ExpenseType: "REWARD",
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInsufficientAvailable, appErr.Code)
	issueRequestDao.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestSubmitIssueRequestSuccess(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	producer := new(mockIssueRequestUpdatedProducer)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, issueBudgetDao, producer)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusDraft,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{AvailableAmount: "100"}, nil).Once()
	issueRequestDao.On("GetByIDForUpdate", mock.Anything, mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusDraft,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{ID: 10, AvailableAmount: "100"}, nil).Once()
	projectBudgetDao.On("LockByID", mock.Anything, mock.Anything, int64(10)).
		Return(&model.ProjectBudget{ID: 10, AvailableAmount: "100"}, nil).Once()
	issueRequestDao.On("UpdateStatusInTx", mock.Anything, mock.Anything, int64(1), model.IssueRequestStatusDraft, model.IssueRequestStatusToApprove, "submit").
		Return(nil).Once()
	projectBudgetDao.On("ApplySubmitWithhold", mock.Anything, mock.Anything, int64(10), "50").Return(nil).Once()

	result, err := svc.SubmitIssueRequest(context.Background(), "doc-1", 1, &data.SubmitIssueRequestRequest{
		Status: model.IssueRequestStatusToApprove,
		Remark: "submit",
	})
	assert.NoError(t, err)
	assert.Equal(t, model.IssueRequestStatusToApprove, result.RequestStatus)
	producer.AssertNotCalled(t, "PublishIssueRequestUpdated", mock.Anything, mock.Anything)
}

func TestApproveIssueRequestRejected(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	producer := new(mockIssueRequestUpdatedProducer)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, issueBudgetDao, producer)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusToApprove,
		}, nil).Once()
	issueRequestDao.On("GetByIDForUpdate", mock.Anything, mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusToApprove,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{ID: 10, WithholdAmount: "50"}, nil).Once()
	projectBudgetDao.On("LockByID", mock.Anything, mock.Anything, int64(10)).
		Return(&model.ProjectBudget{ID: 10, WithholdAmount: "50"}, nil).Once()
	issueRequestDao.On("UpdateStatusInTx", mock.Anything, mock.Anything, int64(1), model.IssueRequestStatusToApprove, model.IssueRequestStatusRejected, "no").
		Return(nil).Once()
	projectBudgetDao.On("ApplyRejectRelease", mock.Anything, mock.Anything, int64(10), "50").Return(nil).Once()

	result, err := svc.ApproveIssueRequest(context.Background(), "doc-1", 1, &data.ApproveIssueRequestRequest{
		Status: model.IssueRequestStatusRejected,
		Remark: "no",
	})
	assert.NoError(t, err)
	assert.Equal(t, model.IssueRequestStatusRejected, result.RequestStatus)
	producer.AssertNotCalled(t, "PublishIssueRequestUpdated", mock.Anything, mock.Anything)
}

func TestProcessKafkaEventInitIssueBudget(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	producer := new(mockIssueRequestUpdatedProducer)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, issueBudgetDao, producer)

	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusApproved,
		}, nil).Once()
	issueRequestDao.On("GetByIDForUpdate", mock.Anything, mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusApproved,
		}, nil).Once()
	issueBudgetDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueBudget")).Return(nil).Once()
	issueRequestDao.On("MarkOngoing", mock.Anything, mock.Anything, int64(1)).Return(nil).Once()

	err := svc.ProcessKafkaEvent(context.Background(), 1)
	assert.NoError(t, err)
}

func TestListIssueRequestsByDocID(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	producer := new(mockIssueRequestUpdatedProducer)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, issueBudgetDao, producer)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("ListByProjectID", mock.Anything, int64(1), 1, 20).
		Return([]*model.IssueRequest{{ID: 1, ProjectID: 1, Amount: "10", RequestStatus: model.IssueRequestStatusDraft}}, int64(1), nil).Once()

	result, err := svc.ListIssueRequestsByDocID(context.Background(), "doc-1", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
}

func TestUpdateIssueRequestSuccess(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	producer := new(mockIssueRequestUpdatedProducer)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, issueBudgetDao, producer)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusDraft,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{AvailableAmount: "100"}, nil).Once()
	issueRequestDao.On("UpdateFields", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT, "60", "updated").
		Return(nil).Once()

	result, err := svc.UpdateIssueRequest(context.Background(), "doc-1", 1, &data.UpdateIssueRequestRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "60",
		Remark:      "updated",
	})
	assert.NoError(t, err)
	assert.Equal(t, "60", result.Amount)
	assert.Equal(t, "updated", result.Remark)
}

func TestUpdateIssueRequestNotEditable(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	producer := new(mockIssueRequestUpdatedProducer)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, issueBudgetDao, producer)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, RequestStatus: model.IssueRequestStatusToApprove,
		}, nil).Once()

	_, err := svc.UpdateIssueRequest(context.Background(), "doc-1", 1, &data.UpdateIssueRequestRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "60",
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidStatusTransition, appErr.Code)
}

func TestApproveIssueRequestApproved(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	producer := new(mockIssueRequestUpdatedProducer)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, issueBudgetDao, producer)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusToApprove,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{WithholdAmount: "50"}, nil).Once()
	issueRequestDao.On("GetByIDForUpdate", mock.Anything, mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusToApprove,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{ID: 10, WithholdAmount: "50"}, nil).Once()
	projectBudgetDao.On("LockByID", mock.Anything, mock.Anything, int64(10)).
		Return(&model.ProjectBudget{ID: 10, WithholdAmount: "50"}, nil).Once()
	issueRequestDao.On("UpdateStatusInTx", mock.Anything, mock.Anything, int64(1), model.IssueRequestStatusToApprove, model.IssueRequestStatusApproved, "ok").
		Return(nil).Once()
	projectBudgetDao.On("ApplyApproveIssued", mock.Anything, mock.Anything, int64(10), "50").Return(nil).Once()
	producer.On("PublishIssueRequestUpdated", mock.Anything, int64(1)).Return(nil).Once()

	result, err := svc.ApproveIssueRequest(context.Background(), "doc-1", 1, &data.ApproveIssueRequestRequest{
		Status: model.IssueRequestStatusApproved,
		Remark: "ok",
	})
	assert.NoError(t, err)
	assert.Equal(t, model.IssueRequestStatusApproved, result.RequestStatus)
}

func TestApproveIssueRequestInsufficientWithhold(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	producer := new(mockIssueRequestUpdatedProducer)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, issueBudgetDao, producer)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusToApprove,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{WithholdAmount: "10"}, nil).Once()

	_, err := svc.ApproveIssueRequest(context.Background(), "doc-1", 1, &data.ApproveIssueRequestRequest{
		Status: model.IssueRequestStatusApproved,
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInsufficientWithhold, appErr.Code)
}

func TestParseIssueRequestID(t *testing.T) {
	id, err := ParseIssueRequestID("42")
	assert.NoError(t, err)
	assert.Equal(t, int64(42), id)

	_, err = ParseIssueRequestID("")
	assert.Error(t, err)

	_, err = ParseIssueRequestID("abc")
	assert.Error(t, err)
}

func TestProcessKafkaEventSkipOngoing(t *testing.T) {
	initServiceTestEnv()
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(new(mockFinanceDocDao), new(mockProjectBudgetDao), issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{ID: 1, RequestStatus: model.IssueRequestStatusOngoing}, nil).Once()

	err := svc.ProcessKafkaEvent(context.Background(), 1)
	assert.NoError(t, err)
}

func TestProcessKafkaEventSkipNonApproved(t *testing.T) {
	initServiceTestEnv()
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(new(mockFinanceDocDao), new(mockProjectBudgetDao), issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{ID: 1, RequestStatus: model.IssueRequestStatusDraft}, nil).Once()

	err := svc.ProcessKafkaEvent(context.Background(), 1)
	assert.NoError(t, err)
}

func TestListIssueRequestsInvalidPagination(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), new(mockIssueRequestDao), new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()

	_, err := svc.ListIssueRequestsByDocID(context.Background(), "doc-1", 0, 20)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidPagination, appErr.Code)
}

func TestCreateIssueRequestInvalidExpenseType(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), new(mockIssueRequestDao), new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()

	_, err := svc.CreateIssueRequest(context.Background(), "doc-1", &data.CreateIssueRequestRequest{
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "10",
		ExpenseType: "INVALID",
	})
	assert.Error(t, err)
}

func TestIssueRequestNotFoundForDoc(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(99)).Return(nil, nil).Once()

	_, err := svc.SubmitIssueRequest(context.Background(), "doc-1", 99, &data.SubmitIssueRequestRequest{
		Status: model.IssueRequestStatusToApprove,
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeIssueRequestNotFound, appErr.Code)
}

func TestSubmitIssueRequestFromRejected(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusRejected,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{AvailableAmount: "100"}, nil).Once()
	issueRequestDao.On("GetByIDForUpdate", mock.Anything, mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusRejected,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{ID: 10, AvailableAmount: "100"}, nil).Once()
	projectBudgetDao.On("LockByID", mock.Anything, mock.Anything, int64(10)).
		Return(&model.ProjectBudget{ID: 10, AvailableAmount: "100"}, nil).Once()
	issueRequestDao.On("UpdateStatusInTx", mock.Anything, mock.Anything, int64(1), model.IssueRequestStatusRejected, model.IssueRequestStatusToApprove, "retry").
		Return(nil).Once()
	projectBudgetDao.On("ApplySubmitWithhold", mock.Anything, mock.Anything, int64(10), "50").Return(nil).Once()

	result, err := svc.SubmitIssueRequest(context.Background(), "doc-1", 1, &data.SubmitIssueRequestRequest{
		Status: model.IssueRequestStatusToApprove,
		Remark: "retry",
	})
	assert.NoError(t, err)
	assert.Equal(t, model.IssueRequestStatusToApprove, result.RequestStatus)
}

func TestSubmitIssueRequestInvalidStatus(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{ID: 1, ProjectID: 1, RequestStatus: model.IssueRequestStatusDraft}, nil).Once()

	_, err := svc.SubmitIssueRequest(context.Background(), "doc-1", 1, &data.SubmitIssueRequestRequest{
		Status: model.IssueRequestStatusApproved,
	})
	assert.Error(t, err)
}

func TestProcessKafkaEventInitIssueBudgetAlreadyExists(t *testing.T) {
	initServiceTestEnv()
	issueRequestDao := new(mockIssueRequestDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	svc := newIssueRequestService(new(mockFinanceDocDao), new(mockProjectBudgetDao), issueRequestDao, issueBudgetDao, new(mockIssueRequestUpdatedProducer))

	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusApproved,
		}, nil).Once()
	issueRequestDao.On("GetByIDForUpdate", mock.Anything, mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusApproved,
		}, nil).Once()
	issueBudgetDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueBudget")).
		Return(dao.ErrIssueBudgetAlreadyExists).Once()
	issueRequestDao.On("MarkOngoing", mock.Anything, mock.Anything, int64(1)).Return(nil).Once()

	err := svc.ProcessKafkaEvent(context.Background(), 1)
	assert.NoError(t, err)
}

func TestCreateIssueRequestNilRequest(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), new(mockIssueRequestDao), new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()

	_, err := svc.CreateIssueRequest(context.Background(), "doc-1", nil)
	assert.Error(t, err)
}

func TestCreateIssueRequestDocNotApproved(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), new(mockIssueRequestDao), new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", Status: model.FinanceDocStatusDraft}, nil).Once()

	_, err := svc.CreateIssueRequest(context.Background(), "doc-1", &data.CreateIssueRequestRequest{
		VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "10", ExpenseType: "REWARD",
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeFinanceDocNotApproved, appErr.Code)
}

func TestApproveIssueRequestUnsupportedStatus(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{ID: 1, ProjectID: 1, RequestStatus: model.IssueRequestStatusToApprove}, nil).Once()

	_, err := svc.ApproveIssueRequest(context.Background(), "doc-1", 1, &data.ApproveIssueRequestRequest{
		Status: model.IssueRequestStatusToApprove,
	})
	assert.Error(t, err)
}

func TestApproveIssueRequestNilRequest(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{ID: 1, ProjectID: 1, RequestStatus: model.IssueRequestStatusToApprove}, nil).Once()

	_, err := svc.ApproveIssueRequest(context.Background(), "doc-1", 1, nil)
	assert.Error(t, err)
}

func TestUpdateIssueRequestNilRequest(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{ID: 1, ProjectID: 1, RequestStatus: model.IssueRequestStatusDraft}, nil).Once()

	_, err := svc.UpdateIssueRequest(context.Background(), "doc-1", 1, nil)
	assert.Error(t, err)
}

func TestProcessKafkaEventNotFound(t *testing.T) {
	initServiceTestEnv()
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(new(mockFinanceDocDao), new(mockProjectBudgetDao), issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	issueRequestDao.On("GetByID", mock.Anything, int64(99)).Return(nil, nil).Once()

	err := svc.ProcessKafkaEvent(context.Background(), 99)
	assert.Error(t, err)
}

func TestIssueRequestWrongProject(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{ID: 1, ProjectID: 2, RequestStatus: model.IssueRequestStatusDraft}, nil).Once()

	_, err := svc.UpdateIssueRequest(context.Background(), "doc-1", 1, &data.UpdateIssueRequestRequest{
		VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "10",
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeIssueRequestNotFound, appErr.Code)
}

func TestCreateIssueRequestBudgetNotFound(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, new(mockIssueRequestDao), new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(nil, nil).Once()

	_, err := svc.CreateIssueRequest(context.Background(), "doc-1", &data.CreateIssueRequestRequest{
		VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "10", ExpenseType: "REWARD",
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeProjectBudgetNotFound, appErr.Code)
}

func TestApproveIssueRequestInvalidTransition(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{ID: 1, ProjectID: 1, RequestStatus: model.IssueRequestStatusDraft}, nil).Once()

	_, err := svc.ApproveIssueRequest(context.Background(), "doc-1", 1, &data.ApproveIssueRequestRequest{
		Status: model.IssueRequestStatusApproved,
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidStatusTransition, appErr.Code)
}

func TestSubmitIssueRequestInsufficientAvailableInTx(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusDraft,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{AvailableAmount: "100"}, nil).Once()
	issueRequestDao.On("GetByIDForUpdate", mock.Anything, mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusDraft,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{ID: 10, AvailableAmount: "10"}, nil).Once()
	projectBudgetDao.On("LockByID", mock.Anything, mock.Anything, int64(10)).
		Return(&model.ProjectBudget{ID: 10, AvailableAmount: "10"}, nil).Once()

	_, err := svc.SubmitIssueRequest(context.Background(), "doc-1", 1, &data.SubmitIssueRequestRequest{
		Status: model.IssueRequestStatusToApprove,
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInsufficientAvailable, appErr.Code)
}

func TestCreateIssueRequestCreateFailed(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{AvailableAmount: "100"}, nil).Once()
	issueRequestDao.On("Create", mock.Anything, mock.AnythingOfType("*model.IssueRequest")).Return(assert.AnError).Once()

	_, err := svc.CreateIssueRequest(context.Background(), "doc-1", &data.CreateIssueRequestRequest{
		VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "10", ExpenseType: "REWARD",
	})
	assert.Error(t, err)
}

func TestUpdateIssueRequestInvalidAmount(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	issueRequestDao := new(mockIssueRequestDao)
	svc := newIssueRequestService(financeDocDao, new(mockProjectBudgetDao), issueRequestDao, new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{ID: 1, ProjectID: 1, RequestStatus: model.IssueRequestStatusDraft}, nil).Once()

	_, err := svc.UpdateIssueRequest(context.Background(), "doc-1", 1, &data.UpdateIssueRequestRequest{
		VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "bad",
	})
	assert.Error(t, err)
}

func TestListIssueRequestsEmptyDocID(t *testing.T) {
	initServiceTestEnv()
	svc := newIssueRequestService(new(mockFinanceDocDao), new(mockProjectBudgetDao), new(mockIssueRequestDao), new(mockIssueBudgetDao), new(mockIssueRequestUpdatedProducer))

	_, err := svc.ListIssueRequestsByDocID(context.Background(), " ", 1, 20)
	assert.Error(t, err)
}

func TestApproveIssueRequestPublishFailure(t *testing.T) {
	initServiceTestEnv()
	financeDocDao := new(mockFinanceDocDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueRequestDao := new(mockIssueRequestDao)
	producer := new(mockIssueRequestUpdatedProducer)
	svc := newIssueRequestService(financeDocDao, projectBudgetDao, issueRequestDao, new(mockIssueBudgetDao), producer)

	financeDocDao.On("GetByDocID", mock.Anything, "doc-1").
		Return(&model.FinanceDoc{DocID: "doc-1", ProjectID: 1, Status: model.FinanceDocStatusApproved}, nil).Once()
	issueRequestDao.On("GetByID", mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusToApprove,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{WithholdAmount: "50"}, nil).Once()
	issueRequestDao.On("GetByIDForUpdate", mock.Anything, mock.Anything, int64(1)).
		Return(&model.IssueRequest{
			ID: 1, ProjectID: 1, VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "50", RequestStatus: model.IssueRequestStatusToApprove,
		}, nil).Once()
	projectBudgetDao.On("GetByProjectIDVoucherTypeUnit", mock.Anything, int64(1), util.VoucherTypeCrypto, util.UnitCryptoUSDT).
		Return(&model.ProjectBudget{ID: 10, WithholdAmount: "50"}, nil).Once()
	projectBudgetDao.On("LockByID", mock.Anything, mock.Anything, int64(10)).
		Return(&model.ProjectBudget{ID: 10, WithholdAmount: "50"}, nil).Once()
	issueRequestDao.On("UpdateStatusInTx", mock.Anything, mock.Anything, int64(1), model.IssueRequestStatusToApprove, model.IssueRequestStatusApproved, "ok").
		Return(nil).Once()
	projectBudgetDao.On("ApplyApproveIssued", mock.Anything, mock.Anything, int64(10), "50").Return(nil).Once()
	producer.On("PublishIssueRequestUpdated", mock.Anything, int64(1)).Return(assert.AnError).Once()

	_, err := svc.ApproveIssueRequest(context.Background(), "doc-1", 1, &data.ApproveIssueRequestRequest{
		Status: model.IssueRequestStatusApproved,
		Remark: "ok",
	})
	assert.Error(t, err)
}
