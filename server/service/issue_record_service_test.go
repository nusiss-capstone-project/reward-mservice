package service

import (
	"context"
	"errors"
	"testing"

	"github.com/nusiss-capstone-project/reward-mservice/common/rewardpb"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/kafka/producer"
	"github.com/nusiss-capstone-project/reward-mservice/server/proxy"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type mockRewardRequestDao struct {
	mock.Mock
}

func (m *mockRewardRequestDao) Create(ctx context.Context, tx *gorm.DB, request *model.RewardRequest) error {
	args := m.Called(ctx, tx, request)
	if args.Error(0) == nil && request.ID == 0 {
		request.ID = 100
	}
	return args.Error(0)
}

func (m *mockRewardRequestDao) GetByClientRefID(ctx context.Context, clientRefID string) (*model.RewardRequest, error) {
	args := m.Called(ctx, clientRefID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.RewardRequest), args.Error(1)
}

func (m *mockRewardRequestDao) GetByID(ctx context.Context, id int64) (*model.RewardRequest, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.RewardRequest), args.Error(1)
}

func (m *mockRewardRequestDao) UpdateStatus(ctx context.Context, tx *gorm.DB, id int64, fromStatus, toStatus string) error {
	args := m.Called(ctx, tx, id, fromStatus, toStatus)
	return args.Error(0)
}

type mockIssueRecordDao struct {
	mock.Mock
}

func (m *mockIssueRecordDao) Create(ctx context.Context, tx *gorm.DB, issueRecord *model.IssueRecord) error {
	args := m.Called(ctx, tx, issueRecord)
	return args.Error(0)
}

func (m *mockIssueRecordDao) Save(ctx context.Context, tx *gorm.DB, issueRecord *model.IssueRecord) error {
	args := m.Called(ctx, tx, issueRecord)
	return args.Error(0)
}

func (m *mockIssueRecordDao) GetByID(ctx context.Context, id int64) (*model.IssueRecord, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.IssueRecord), args.Error(1)
}

func (m *mockIssueRecordDao) GetByClientRefId(ctx context.Context, clientRefID string) (*model.IssueRecord, error) {
	args := m.Called(ctx, clientRefID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.IssueRecord), args.Error(1)
}

type mockRewardExecuteProducer struct {
	mock.Mock
}

func (m *mockRewardExecuteProducer) PublishExecute(ctx context.Context, rewardRequestID int64) error {
	args := m.Called(ctx, rewardRequestID)
	return args.Error(0)
}

type mockRewardResultProducer struct {
	mock.Mock
}

func (m *mockRewardResultProducer) PublishResult(ctx context.Context, event producer.RewardDistributionResultEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

type mockRiskChecker struct {
	mock.Mock
}

func (m *mockRiskChecker) Check(ctx context.Context, userID int64) (bool, string) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.String(1)
}

type mockVoucherIssuer struct {
	mock.Mock
}

func (m *mockVoucherIssuer) Issue(
	ctx context.Context,
	record *proxy.IssueRecordSnapshot,
	amount string,
) (bool, string, error) {
	args := m.Called(ctx, record, amount)
	return args.Bool(0), args.String(1), args.Error(2)
}

func newIssueRecordTestService(
	rewardRequestDao *mockRewardRequestDao,
	issueRecordDao *mockIssueRecordDao,
	projectDao *mockProjectDao,
	projectBudgetDao *mockProjectBudgetDao,
	issueBudgetDao *mockIssueBudgetDao,
	executeProducer *mockRewardExecuteProducer,
	resultProducer *mockRewardResultProducer,
	riskChecker *mockRiskChecker,
	voucherIssuer *mockVoucherIssuer,
) *IssueRecordServiceImpl {
	return &IssueRecordServiceImpl{
		rewardRequestDao: rewardRequestDao,
		issueRecordDao:   issueRecordDao,
		projectDao:       projectDao,
		projectBudgetDao: projectBudgetDao,
		issueBudgetDao:   issueBudgetDao,
		executeProducer:  executeProducer,
		resultProducer:   resultProducer,
		riskChecker:      riskChecker,
		voucherIssuer:    voucherIssuer,
		txBeginner:       passthroughTxBeginner{},
	}
}

func validRewardRequest() *rewardpb.RewardDistributionRequest {
	return &rewardpb.RewardDistributionRequest{
		ClientRefId: "ref-1",
		UserId:      10,
		ProjectId:   20,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "1.0",
	}
}

func mockProcessBudgetExists(projectBudgetDao *mockProjectBudgetDao) {
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
	).Return(&model.ProjectBudget{ID: 50}, nil).Once()
}

func TestProcessVoucherIssueRequestSuccess(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	executeProducer := new(mockRewardExecuteProducer)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mockIssueRecordDao), new(mockProjectDao),
		projectBudgetDao, new(mockIssueBudgetDao),
		executeProducer, new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	mockProcessBudgetExists(projectBudgetDao)
	rewardRequestDao.On("GetByClientRefID", mock.Anything, "ref-1").Return(nil, nil)
	rewardRequestDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.RewardRequest")).Return(nil)
	executeProducer.On("PublishExecute", mock.Anything, int64(100)).Return(nil)

	err := svc.ProcessVoucherIssueRequest(context.Background(), validRewardRequest())
	assert.NoError(t, err)
	mock.AssertExpectationsForObjects(t, rewardRequestDao, executeProducer, projectBudgetDao)
}

func TestProcessVoucherIssueRequestDuplicateClientRefID(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mockIssueRecordDao), new(mockProjectDao),
		projectBudgetDao, new(mockIssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	mockProcessBudgetExists(projectBudgetDao)
	rewardRequestDao.On("GetByClientRefID", mock.Anything, "ref-1").Return(&model.RewardRequest{ID: 1}, nil)

	err := svc.ProcessVoucherIssueRequest(context.Background(), validRewardRequest())
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeDuplicateClientRefID, appErr.Code)
}

func TestExecuteRewardDistributionRiskFailure(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	issueRecordDao := new(mockIssueRecordDao)
	resultProducer := new(mockRewardResultProducer)
	riskChecker := new(mockRiskChecker)

	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mockProjectDao),
		new(mockProjectBudgetDao), new(mockIssueBudgetDao),
		new(mockRewardExecuteProducer), resultProducer,
		riskChecker, new(mockVoucherIssuer),
	)

	rewardRequest := &model.RewardRequest{
		ID:          100,
		ClientRefID: "ref-1",
		UserID:      10,
		ProjectID:   20,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "1.0",
		Status:      model.RewardRequestStatusPending,
	}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(rewardRequest, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-1").Return(nil, nil)
	riskChecker.On("Check", mock.Anything, int64(10)).Return(false, "blocked by risk")
	issueRecordDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	rewardRequestDao.On("UpdateStatus", mock.Anything, mock.Anything, int64(100), model.RewardRequestStatusPending, model.RewardRequestStatusCompleted).Return(nil)
	resultProducer.On("PublishResult", mock.Anything, mock.MatchedBy(func(event producer.RewardDistributionResultEvent) bool {
		return event.ClientRefID == "ref-1" &&
			event.Status == distributionResultStatusFailed &&
			event.DistributedAmount == "0" &&
			event.FailedReason == "blocked by risk"
	})).Return(nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
}

func TestExecuteRewardDistributionDeferWhenBudgetInsufficient(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	riskChecker := new(mockRiskChecker)

	svc := newIssueRecordTestService(
		rewardRequestDao, new(mockIssueRecordDao), new(mockProjectDao),
		projectBudgetDao, issueBudgetDao,
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		riskChecker, new(mockVoucherIssuer),
	)

	rewardRequest := &model.RewardRequest{
		ID:          100,
		ClientRefID: "ref-defer",
		ProjectID:   20,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "1.0",
		Status:      model.RewardRequestStatusPending,
	}
	projectBudget := &model.ProjectBudget{ID: 50, AvailableAmount: "10.0"}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(rewardRequest, nil)
	issueRecordDao := new(mockIssueRecordDao)
	svc.issueRecordDao = issueRecordDao
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-defer").Return(nil, nil)
	riskChecker.On("Check", mock.Anything, mock.Anything).Return(true, "")
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
	).Return(projectBudget, nil).Once()
	issueBudgetDao.On(
		"GetFirstAvailableForDistribution",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
		"1.0",
	).Return(nil, nil).Once()

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.ErrorIs(t, err, ErrDistributionDeferred)
}

func TestExecuteRewardDistributionSuccess(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	issueRecordDao := new(mockIssueRecordDao)
	projectDao := new(mockProjectDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	resultProducer := new(mockRewardResultProducer)
	riskChecker := new(mockRiskChecker)
	voucherIssuer := new(mockVoucherIssuer)

	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, projectDao,
		projectBudgetDao, issueBudgetDao,
		new(mockRewardExecuteProducer), resultProducer,
		riskChecker, voucherIssuer,
	)

	rewardRequest := &model.RewardRequest{
		ID:          100,
		ClientRefID: "ref-1",
		UserID:      10,
		ProjectID:   20,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "1.0",
		Status:      model.RewardRequestStatusPending,
	}
	issueBudget := &model.IssueBudget{ID: 30, IssueRequestID: 40, AvailableAmount: "10.0"}
	projectBudget := &model.ProjectBudget{ID: 50, AvailableAmount: "10.0"}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(rewardRequest, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-1").Return(nil, nil).Once()
	riskChecker.On("Check", mock.Anything, int64(10)).Return(true, "")
	issueBudgetDao.On(
		"GetFirstAvailableForDistribution",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
		"1.0",
	).Return(issueBudget, nil).Once()
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
	).Return(projectBudget, nil).Once()
	projectBudgetDao.On("ApplyDistributionDeductAvailable", mock.Anything, mock.Anything, int64(50), "1.0").Return(nil)
	issueBudgetDao.On("ApplyDistributionDeductAvailable", mock.Anything, mock.Anything, int64(30), "1.0").Return(nil)
	issueRecordDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	voucherIssuer.On("Issue", mock.Anything, mock.Anything, "1.0").Return(true, "", nil)
	projectBudgetDao.On("ApplyDistributionIssued", mock.Anything, mock.Anything, int64(50), "1.0").Return(nil)
	issueBudgetDao.On("ApplyDistributionIssued", mock.Anything, mock.Anything, int64(30), "1.0").Return(nil)
	issueRecordDao.On("Save", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	rewardRequestDao.On("UpdateStatus", mock.Anything, mock.Anything, int64(100), model.RewardRequestStatusPending, model.RewardRequestStatusCompleted).Return(nil)
	resultProducer.On("PublishResult", mock.Anything, mock.MatchedBy(func(event producer.RewardDistributionResultEvent) bool {
		return event.ClientRefID == "ref-1" &&
			event.Status == distributionResultStatusDistributed &&
			event.DistributedAmount == "1.0"
	})).Return(nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
}

func TestExecuteRewardDistributionDownstreamCallFailRetry(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	issueRecordDao := new(mockIssueRecordDao)
	projectDao := new(mockProjectDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	riskChecker := new(mockRiskChecker)
	voucherIssuer := new(mockVoucherIssuer)

	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, projectDao,
		projectBudgetDao, issueBudgetDao,
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		riskChecker, voucherIssuer,
	)

	rewardRequest := &model.RewardRequest{
		ID:          100,
		ClientRefID: "ref-retry",
		UserID:      10,
		ProjectID:   20,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "1.0",
		Status:      model.RewardRequestStatusPending,
	}
	issueBudget := &model.IssueBudget{ID: 30, IssueRequestID: 40, AvailableAmount: "10.0"}
	projectBudget := &model.ProjectBudget{ID: 50, AvailableAmount: "10.0"}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(rewardRequest, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-retry").Return(nil, nil).Once()
	riskChecker.On("Check", mock.Anything, int64(10)).Return(true, "")
	issueBudgetDao.On(
		"GetFirstAvailableForDistribution",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
		"1.0",
	).Return(issueBudget, nil).Once()
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
	).Return(projectBudget, nil).Once()
	projectBudgetDao.On("ApplyDistributionDeductAvailable", mock.Anything, mock.Anything, int64(50), "1.0").Return(nil)
	issueBudgetDao.On("ApplyDistributionDeductAvailable", mock.Anything, mock.Anything, int64(30), "1.0").Return(nil)
	issueRecordDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	voucherIssuer.On("Issue", mock.Anything, mock.Anything, "1.0").
		Return(false, "", errors.New("network error"))

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.ErrorIs(t, err, ErrDistributionRetry)
}

func TestExecuteRewardDistributionRetryWithExistingPendingRecord(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	issueRecordDao := new(mockIssueRecordDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	resultProducer := new(mockRewardResultProducer)
	voucherIssuer := new(mockVoucherIssuer)

	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mockProjectDao),
		projectBudgetDao, issueBudgetDao,
		new(mockRewardExecuteProducer), resultProducer,
		new(mockRiskChecker), voucherIssuer,
	)

	issueRequestID := int64(40)
	rewardRequest := &model.RewardRequest{
		ID:          100,
		ClientRefID: "ref-retry-existing",
		UserID:      10,
		ProjectID:   20,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "1.0",
		Status:      model.RewardRequestStatusPending,
	}
	existingRecord := &model.IssueRecord{
		ID:                200,
		IssueRequestID:    &issueRequestID,
		ProjectID:         20,
		UserID:            10,
		VoucherType:       util.VoucherTypeCrypto,
		Unit:              util.UnitCryptoUSDT,
		VoucherID:         "voucher-retry",
		IssueStatus:       model.IssueRecordStatusPending,
		ClientReferenceID: "ref-retry-existing",
	}
	projectBudget := &model.ProjectBudget{ID: 50, AvailableAmount: "10.0"}
	issueBudget := &model.IssueBudget{ID: 30, IssueRequestID: 40, AvailableAmount: "10.0"}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(rewardRequest, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-retry-existing").Return(existingRecord, nil).Once()
	issueRecordDao.On("GetByID", mock.Anything, int64(200)).Return(existingRecord, nil).Once()
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
	).Return(projectBudget, nil).Once()
	projectBudgetDao.On("GetByID", mock.Anything, int64(50)).Return(projectBudget, nil).Once()
	issueBudgetDao.On("GetByIssueRequestID", mock.Anything, int64(40)).Return(issueBudget, nil).Once()
	issueBudgetDao.On("GetByID", mock.Anything, int64(30)).Return(issueBudget, nil).Once()
	voucherIssuer.On("Issue", mock.Anything, mock.Anything, "1.0").Return(true, "", nil)
	projectBudgetDao.On("ApplyDistributionIssued", mock.Anything, mock.Anything, int64(50), "1.0").Return(nil)
	issueBudgetDao.On("ApplyDistributionIssued", mock.Anything, mock.Anything, int64(30), "1.0").Return(nil)
	issueRecordDao.On("Save", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	rewardRequestDao.On(
		"UpdateStatus", mock.Anything, mock.Anything, int64(100),
		model.RewardRequestStatusPending, model.RewardRequestStatusCompleted,
	).Return(nil)
	resultProducer.On("PublishResult", mock.Anything, mock.MatchedBy(func(event producer.RewardDistributionResultEvent) bool {
		return event.ClientRefID == "ref-retry-existing" &&
			event.VoucherID == "voucher-retry" &&
			event.Status == distributionResultStatusDistributed
	})).Return(nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
	issueBudgetDao.AssertNotCalled(t, "GetFirstAvailableForDistribution", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	issueRecordDao.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func TestExecuteRewardDistributionBusinessFailureRefund(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	issueRecordDao := new(mockIssueRecordDao)
	projectDao := new(mockProjectDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	resultProducer := new(mockRewardResultProducer)
	riskChecker := new(mockRiskChecker)
	voucherIssuer := new(mockVoucherIssuer)

	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, projectDao,
		projectBudgetDao, issueBudgetDao,
		new(mockRewardExecuteProducer), resultProducer,
		riskChecker, voucherIssuer,
	)

	rewardRequest := &model.RewardRequest{
		ID:          100,
		ClientRefID: "ref-1",
		UserID:      10,
		ProjectID:   20,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "1.0",
		Status:      model.RewardRequestStatusPending,
	}
	issueBudget := &model.IssueBudget{ID: 30, IssueRequestID: 40, AvailableAmount: "10.0"}
	projectBudget := &model.ProjectBudget{ID: 50, AvailableAmount: "10.0"}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(rewardRequest, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-1").Return(nil, nil).Once()
	riskChecker.On("Check", mock.Anything, int64(10)).Return(true, "")
	issueBudgetDao.On(
		"GetFirstAvailableForDistribution",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
		"1.0",
	).Return(issueBudget, nil).Once()
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
	).Return(projectBudget, nil).Once()
	projectBudgetDao.On("ApplyDistributionDeductAvailable", mock.Anything, mock.Anything, int64(50), "1.0").Return(nil)
	issueBudgetDao.On("ApplyDistributionDeductAvailable", mock.Anything, mock.Anything, int64(30), "1.0").Return(nil)
	issueRecordDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	voucherIssuer.On("Issue", mock.Anything, mock.Anything, "1.0").
		Return(false, "asset rejected", nil)
	projectBudgetDao.On("ApplyDistributionRefund", mock.Anything, mock.Anything, int64(50), "1.0").Return(nil)
	issueBudgetDao.On("ApplyDistributionRefund", mock.Anything, mock.Anything, int64(30), "1.0").Return(nil)
	issueRecordDao.On("Save", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	rewardRequestDao.On("UpdateStatus", mock.Anything, mock.Anything, int64(100), model.RewardRequestStatusPending, model.RewardRequestStatusCompleted).Return(nil)
	resultProducer.On("PublishResult", mock.Anything, mock.MatchedBy(func(event producer.RewardDistributionResultEvent) bool {
		return event.Status == distributionResultStatusFailed &&
			event.FailedReason == "asset rejected"
	})).Return(nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
}

func TestProcessVoucherIssueRequestNilRequest(t *testing.T) {
	initServiceTestEnv()
	svc := newIssueRecordTestService(
		new(mockRewardRequestDao), new(mockIssueRecordDao), new(mockProjectDao),
		new(mockProjectBudgetDao), new(mockIssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	err := svc.ProcessVoucherIssueRequest(context.Background(), nil)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestProcessVoucherIssueRequestBudgetNotFound(t *testing.T) {
	initServiceTestEnv()
	projectBudgetDao := new(mockProjectBudgetDao)
	svc := newIssueRecordTestService(
		new(mockRewardRequestDao), new(mockIssueRecordDao), new(mockProjectDao),
		projectBudgetDao, new(mockIssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
	).Return(nil, nil).Once()

	err := svc.ProcessVoucherIssueRequest(context.Background(), validRewardRequest())
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestProcessVoucherIssueRequestPublishExecuteFailed(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	executeProducer := new(mockRewardExecuteProducer)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mockIssueRecordDao), new(mockProjectDao),
		projectBudgetDao, new(mockIssueBudgetDao),
		executeProducer, new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	mockProcessBudgetExists(projectBudgetDao)
	rewardRequestDao.On("GetByClientRefID", mock.Anything, "ref-1").Return(nil, nil)
	rewardRequestDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.RewardRequest")).Return(nil)
	executeProducer.On("PublishExecute", mock.Anything, int64(100)).Return(errors.New("mq down"))

	err := svc.ProcessVoucherIssueRequest(context.Background(), validRewardRequest())
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInternalError, appErr.Code)
}

func TestExecuteRewardDistributionEnsureCompletedAndSkip(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	issueRecordDao := new(mockIssueRecordDao)
	resultProducer := new(mockRewardResultProducer)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mockProjectDao),
		new(mockProjectBudgetDao), new(mockIssueBudgetDao),
		new(mockRewardExecuteProducer), resultProducer,
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	rewardRequest := &model.RewardRequest{
		ID:          100,
		ClientRefID: "ref-skip",
		Status:      model.RewardRequestStatusPending,
	}
	existingRecord := &model.IssueRecord{
		VoucherID:    "voucher-done",
		IssueStatus:  model.IssueRecordStatusIssued,
		RewardAmount: "1.0",
	}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(rewardRequest, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-skip").Return(existingRecord, nil)
	rewardRequestDao.On("UpdateStatus", mock.Anything, mock.Anything, int64(100), model.RewardRequestStatusPending, model.RewardRequestStatusCompleted).Return(nil)
	resultProducer.On("PublishResult", mock.Anything, mock.MatchedBy(func(event producer.RewardDistributionResultEvent) bool {
		return event.ClientRefID == "ref-skip" &&
			event.VoucherID == "voucher-done" &&
			event.Status == distributionResultStatusDistributed &&
			event.DistributedAmount == "1.0"
	})).Return(nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
}

func TestProcessVoucherIssueRequestValidation(t *testing.T) {
	initServiceTestEnv()
	svc := newIssueRecordTestService(
		new(mockRewardRequestDao), new(mockIssueRecordDao), new(mockProjectDao),
		new(mockProjectBudgetDao), new(mockIssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	tests := []struct {
		name    string
		request *rewardpb.RewardDistributionRequest
	}{
		{
			name: "empty client_ref_id",
			request: &rewardpb.RewardDistributionRequest{
				UserId: 10, ProjectId: 20,
				VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "1.0",
			},
		},
		{
			name: "missing user_id",
			request: &rewardpb.RewardDistributionRequest{
				ClientRefId: "ref-1", ProjectId: 20,
				VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "1.0",
			},
		},
		{
			name: "missing project_id",
			request: &rewardpb.RewardDistributionRequest{
				ClientRefId: "ref-1", UserId: 10,
				VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "1.0",
			},
		},
		{
			name: "invalid voucher_type",
			request: &rewardpb.RewardDistributionRequest{
				ClientRefId: "ref-1", UserId: 10, ProjectId: 20,
				VoucherType: "BAD_TYPE", Unit: util.UnitCryptoUSDT, Amount: "1.0",
			},
		},
		{
			name: "invalid unit",
			request: &rewardpb.RewardDistributionRequest{
				ClientRefId: "ref-1", UserId: 10, ProjectId: 20,
				VoucherType: util.VoucherTypeCrypto, Unit: "BAD_UNIT", Amount: "1.0",
			},
		},
		{
			name: "invalid amount",
			request: &rewardpb.RewardDistributionRequest{
				ClientRefId: "ref-1", UserId: 10, ProjectId: 20,
				VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "bad",
			},
		},
		{
			name: "non positive amount",
			request: &rewardpb.RewardDistributionRequest{
				ClientRefId: "ref-1", UserId: 10, ProjectId: 20,
				VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT, Amount: "0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ProcessVoucherIssueRequest(context.Background(), tt.request)
			assert.Error(t, err)
			var appErr *errs.AppError
			assert.True(t, errors.As(err, &appErr))
			assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
		})
	}
}

func TestExecuteRewardDistributionEnsureCompletedAlreadyCompleted(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	issueRecordDao := new(mockIssueRecordDao)
	resultProducer := new(mockRewardResultProducer)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mockProjectDao),
		new(mockProjectBudgetDao), new(mockIssueBudgetDao),
		new(mockRewardExecuteProducer), resultProducer,
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	rewardRequest := &model.RewardRequest{
		ID:          100,
		ClientRefID: "ref-skip",
		Status:      model.RewardRequestStatusPending,
	}
	existingRecord := &model.IssueRecord{
		VoucherID:   "voucher-done",
		IssueStatus: model.IssueRecordStatusIssued,
	}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(rewardRequest, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-skip").Return(existingRecord, nil)
	rewardRequestDao.On(
		"UpdateStatus", mock.Anything, mock.Anything, int64(100),
		model.RewardRequestStatusPending, model.RewardRequestStatusCompleted,
	).Return(errs.New(errs.CodeInvalidStatusTransition, "reward request status transition failed"))

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
	resultProducer.AssertNotCalled(t, "PublishResult", mock.Anything, mock.Anything)
}

func TestExecuteRewardDistributionFinalizeAlreadyCompleted(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mockRewardRequestDao)
	issueRecordDao := new(mockIssueRecordDao)
	projectBudgetDao := new(mockProjectBudgetDao)
	issueBudgetDao := new(mockIssueBudgetDao)
	resultProducer := new(mockRewardResultProducer)
	riskChecker := new(mockRiskChecker)
	voucherIssuer := new(mockVoucherIssuer)

	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mockProjectDao),
		projectBudgetDao, issueBudgetDao,
		new(mockRewardExecuteProducer), resultProducer,
		riskChecker, voucherIssuer,
	)

	rewardRequest := &model.RewardRequest{
		ID:          100,
		ClientRefID: "ref-dup",
		UserID:      10,
		ProjectID:   20,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Amount:      "1.0",
		Status:      model.RewardRequestStatusPending,
	}
	issueBudget := &model.IssueBudget{ID: 30, IssueRequestID: 40, AvailableAmount: "10.0"}
	projectBudget := &model.ProjectBudget{ID: 50, AvailableAmount: "10.0"}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(rewardRequest, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-dup").Return(nil, nil).Once()
	riskChecker.On("Check", mock.Anything, int64(10)).Return(true, "")
	issueBudgetDao.On(
		"GetFirstAvailableForDistribution",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
		"1.0",
	).Return(issueBudget, nil).Once()
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
	).Return(projectBudget, nil).Once()
	projectBudgetDao.On("ApplyDistributionDeductAvailable", mock.Anything, mock.Anything, int64(50), "1.0").Return(nil)
	issueBudgetDao.On("ApplyDistributionDeductAvailable", mock.Anything, mock.Anything, int64(30), "1.0").Return(nil)
	issueRecordDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	voucherIssuer.On("Issue", mock.Anything, mock.Anything, "1.0").Return(true, "", nil)
	projectBudgetDao.On("ApplyDistributionIssued", mock.Anything, mock.Anything, int64(50), "1.0").Return(nil)
	issueBudgetDao.On("ApplyDistributionIssued", mock.Anything, mock.Anything, int64(30), "1.0").Return(nil)
	issueRecordDao.On("Save", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	rewardRequestDao.On(
		"UpdateStatus", mock.Anything, mock.Anything, int64(100),
		model.RewardRequestStatusPending, model.RewardRequestStatusCompleted,
	).Return(errs.New(errs.CodeInvalidStatusTransition, "reward request status transition failed"))

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
	resultProducer.AssertNotCalled(t, "PublishResult", mock.Anything, mock.Anything)
}
