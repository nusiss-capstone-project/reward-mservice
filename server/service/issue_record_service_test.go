package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/nusiss-capstone-project/reward-mservice/common/rewardpb"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/kafka/producer"
	"github.com/nusiss-capstone-project/reward-mservice/server/proxy"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao/mocks"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
	rewardRequestDao *mocks.RewardRequestDao,
	issueRecordDao *mocks.IssueRecordDao,
	projectDao *mocks.ProjectDao,
	projectBudgetDao *mocks.ProjectBudgetDao,
	issueBudgetDao *mocks.IssueBudgetDao,
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

func mockProcessBudgetExists(projectBudgetDao *mocks.ProjectBudgetDao) {
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit",
		mock.Anything, int64(20),
		util.VoucherTypeCrypto,
		util.UnitCryptoUSDT,
	).Return(&model.ProjectBudget{ID: 50}, nil).Once()
}

// mockRewardRequestCreateAssignID mirrors the old hand-written Create mock,
// which assigned ID=100 when Create succeeds with ID unset (e.g. after DB insert).
func mockRewardRequestCreateAssignID(rewardRequestDao *mocks.RewardRequestDao) *mock.Call {
	return rewardRequestDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.RewardRequest")).
		Run(func(args mock.Arguments) {
			req := args.Get(2).(*model.RewardRequest)
			if req.ID == 0 {
				req.ID = 100
			}
		})
}

func TestProcessVoucherIssueRequestSuccess(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	executeProducer := new(mockRewardExecuteProducer)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
		executeProducer, new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	mockProcessBudgetExists(projectBudgetDao)
	rewardRequestDao.On("GetByClientRefID", mock.Anything, "ref-1").Return(nil, nil)
	mockRewardRequestCreateAssignID(rewardRequestDao).Return(nil)
	executeProducer.On("PublishExecute", mock.Anything, int64(100)).Return(nil)

	err := svc.ProcessVoucherIssueRequest(context.Background(), validRewardRequest())
	assert.NoError(t, err)
	mock.AssertExpectationsForObjects(t, rewardRequestDao, executeProducer, projectBudgetDao)
}

func TestProcessVoucherIssueRequestDuplicateClientRefID(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
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

func TestProcessVoucherIssueRequestWithFixedTemplate(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	templateDao := new(mocks.TemplateDao)
	executeProducer := new(mockRewardExecuteProducer)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
		executeProducer, new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)
	svc.templateDao = templateDao

	fixConfig, _ := json.Marshal(model.FixTemplateConfig{Amount: "2.5"})
	templateDao.On("GetByID", mock.Anything, int64(7)).Return(&model.Template{
		ID:          7,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        model.TemplateTypeFixed,
		Config:      fixConfig,
		Status:      model.TemplateStatusPublished,
	}, nil).Once()
	mockProcessBudgetExists(projectBudgetDao)
	rewardRequestDao.On("GetByClientRefID", mock.Anything, "ref-1").Return(nil, nil)
	mockRewardRequestCreateAssignID(rewardRequestDao).Run(func(args mock.Arguments) {
		req := args.Get(2).(*model.RewardRequest)
		assert.Equal(t, "2.50000000", req.Amount)
		assert.Equal(t, util.VoucherTypeCrypto, req.VoucherType)
		assert.Equal(t, util.UnitCryptoUSDT, req.Unit)
		if req.ID == 0 {
			req.ID = 100
		}
	}).Return(nil)
	executeProducer.On("PublishExecute", mock.Anything, int64(100)).Return(nil)

	err := svc.ProcessVoucherIssueRequest(context.Background(), &rewardpb.RewardDistributionRequest{
		ClientRefId: "ref-1",
		UserId:      10,
		ProjectId:   20,
		TemplateId:  7,
	})
	assert.NoError(t, err)
}

func TestProcessVoucherIssueRequestWithDynamicTemplate(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	templateDao := new(mocks.TemplateDao)
	executeProducer := new(mockRewardExecuteProducer)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
		executeProducer, new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)
	svc.templateDao = templateDao

	dynamicConfig, _ := json.Marshal(model.DynamicTemplateConfig{
		BaseMetric: "net_deposit", Rate: 0.1, Cap: "5",
	})
	templateDao.On("GetByID", mock.Anything, int64(8)).Return(&model.Template{
		ID:          8,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        model.TemplateTypeDynamic,
		Config:      dynamicConfig,
		Status:      model.TemplateStatusPublished,
	}, nil).Once()
	mockProcessBudgetExists(projectBudgetDao)
	rewardRequestDao.On("GetByClientRefID", mock.Anything, "ref-1").Return(nil, nil)
	mockRewardRequestCreateAssignID(rewardRequestDao).Run(func(args mock.Arguments) {
		req := args.Get(2).(*model.RewardRequest)
		assert.Equal(t, "5.00000000", req.Amount) // 100*0.1=10 capped to 5
		if req.ID == 0 {
			req.ID = 100
		}
	}).Return(nil)
	executeProducer.On("PublishExecute", mock.Anything, int64(100)).Return(nil)

	err := svc.ProcessVoucherIssueRequest(context.Background(), &rewardpb.RewardDistributionRequest{
		ClientRefId: "ref-1",
		UserId:      10,
		ProjectId:   20,
		TemplateId:  8,
		Metrics:     map[string]string{"net_deposit": "100"},
	})
	assert.NoError(t, err)
}

func TestProcessVoucherIssueRequestTemplateAmountZero(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	rewardRequestDao := new(mocks.RewardRequestDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)
	svc.templateDao = templateDao

	dynamicConfig, _ := json.Marshal(model.DynamicTemplateConfig{
		BaseMetric: "net_deposit", Rate: 0.1,
	})
	templateDao.On("GetByID", mock.Anything, int64(9)).Return(&model.Template{
		ID:          9,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
		Type:        model.TemplateTypeDynamic,
		Config:      dynamicConfig,
		Status:      model.TemplateStatusPublished,
	}, nil).Once()

	err := svc.ProcessVoucherIssueRequest(context.Background(), &rewardpb.RewardDistributionRequest{
		ClientRefId: "ref-1",
		UserId:      10,
		ProjectId:   20,
		TemplateId:  9,
		Metrics:     map[string]string{"net_deposit": "0"},
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
	rewardRequestDao.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessVoucherIssueRequestAmountZero(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	req := validRewardRequest()
	req.Amount = "0"
	err := svc.ProcessVoucherIssueRequest(context.Background(), req)
	assert.Error(t, err)
	rewardRequestDao.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func TestExecuteRewardDistributionRiskFailure(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	resultProducer := new(mockRewardResultProducer)
	riskChecker := new(mockRiskChecker)

	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
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
	rewardRequestDao := new(mocks.RewardRequestDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
	riskChecker := new(mockRiskChecker)

	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
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
	issueRecordDao := new(mocks.IssueRecordDao)
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
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	projectDao := new(mocks.ProjectDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
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
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	projectDao := new(mocks.ProjectDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
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
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
	resultProducer := new(mockRewardResultProducer)
	voucherIssuer := new(mockVoucherIssuer)

	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
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
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	projectDao := new(mocks.ProjectDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
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
		new(mocks.RewardRequestDao), new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
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
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := newIssueRecordTestService(
		new(mocks.RewardRequestDao), new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
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
	rewardRequestDao := new(mocks.RewardRequestDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	executeProducer := new(mockRewardExecuteProducer)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
		executeProducer, new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	mockProcessBudgetExists(projectBudgetDao)
	rewardRequestDao.On("GetByClientRefID", mock.Anything, "ref-1").Return(nil, nil)
	mockRewardRequestCreateAssignID(rewardRequestDao).Return(nil)
	executeProducer.On("PublishExecute", mock.Anything, int64(100)).Return(errors.New("mq down"))

	err := svc.ProcessVoucherIssueRequest(context.Background(), validRewardRequest())
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInternalError, appErr.Code)
}

func TestExecuteRewardDistributionEnsureCompletedAndSkip(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	resultProducer := new(mockRewardResultProducer)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
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
		new(mocks.RewardRequestDao), new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
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
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	resultProducer := new(mockRewardResultProducer)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
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
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
	resultProducer := new(mockRewardResultProducer)
	riskChecker := new(mockRiskChecker)
	voucherIssuer := new(mockVoucherIssuer)

	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
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

func TestExecuteRewardDistributionInvalidRewardRequestID(t *testing.T) {
	initServiceTestEnv()
	svc := newIssueRecordTestService(
		new(mocks.RewardRequestDao), new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	err := svc.ExecuteRewardDistribution(context.Background(), 0)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestExecuteRewardDistributionRewardRequestNotFound(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(nil, nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
}

func TestExecuteRewardDistributionRewardRequestNotPending(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(&model.RewardRequest{
		ID:     100,
		Status: model.RewardRequestStatusCompleted,
	}, nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
}

func TestExecuteRewardDistributionGetRewardRequestFailed(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(nil, errors.New("db down"))

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.Error(t, err)
}

func TestExecuteRewardDistributionGetIssueRecordFailed(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(&model.RewardRequest{
		ID:          100,
		ClientRefID: "ref-err",
		Status:      model.RewardRequestStatusPending,
	}, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-err").Return(nil, errors.New("db down"))

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.Error(t, err)
}

func TestExecuteRewardDistributionExistingRecordNotFound(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	existingRecord := &model.IssueRecord{ID: 200, IssueStatus: model.IssueRecordStatusPending}
	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(&model.RewardRequest{
		ID: 100, ClientRefID: "ref-missing", Status: model.RewardRequestStatusPending,
	}, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-missing").Return(existingRecord, nil)
	issueRecordDao.On("GetByID", mock.Anything, int64(200)).Return(nil, nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestExecuteRewardDistributionExistingRecordIssueRequestIDMissing(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	existingRecord := &model.IssueRecord{
		ID: 200, IssueStatus: model.IssueRecordStatusPending, ClientReferenceID: "ref-no-issue",
	}
	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(&model.RewardRequest{
		ID: 100, ClientRefID: "ref-no-issue", Status: model.RewardRequestStatusPending,
	}, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-no-issue").Return(existingRecord, nil)
	issueRecordDao.On("GetByID", mock.Anything, int64(200)).Return(existingRecord, nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestExecuteRewardDistributionExistingRecordProjectBudgetMissing(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	issueRequestID := int64(40)
	existingRecord := &model.IssueRecord{
		ID: 200, ProjectID: 20, IssueRequestID: &issueRequestID,
		VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT,
		IssueStatus: model.IssueRecordStatusPending, ClientReferenceID: "ref-no-pb",
	}
	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(&model.RewardRequest{
		ID: 100, ClientRefID: "ref-no-pb", Status: model.RewardRequestStatusPending,
	}, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-no-pb").Return(existingRecord, nil)
	issueRecordDao.On("GetByID", mock.Anything, int64(200)).Return(existingRecord, nil)
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit", mock.Anything, int64(20),
		util.VoucherTypeCrypto, util.UnitCryptoUSDT,
	).Return(nil, nil)
	rewardRequestDao.On(
		"UpdateStatus", mock.Anything, mock.Anything, int64(100),
		model.RewardRequestStatusPending, model.RewardRequestStatusCompleted,
	).Return(nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
}

func TestExecuteRewardDistributionExistingRecordIssueBudgetMissing(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		projectBudgetDao, issueBudgetDao,
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	issueRequestID := int64(40)
	existingRecord := &model.IssueRecord{
		ID: 200, ProjectID: 20, IssueRequestID: &issueRequestID,
		VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT,
		IssueStatus: model.IssueRecordStatusPending, ClientReferenceID: "ref-no-ib",
	}
	projectBudget := &model.ProjectBudget{ID: 50}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(&model.RewardRequest{
		ID: 100, ClientRefID: "ref-no-ib", Status: model.RewardRequestStatusPending,
	}, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-no-ib").Return(existingRecord, nil)
	issueRecordDao.On("GetByID", mock.Anything, int64(200)).Return(existingRecord, nil)
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit", mock.Anything, int64(20),
		util.VoucherTypeCrypto, util.UnitCryptoUSDT,
	).Return(projectBudget, nil)
	projectBudgetDao.On("GetByID", mock.Anything, int64(50)).Return(projectBudget, nil)
	issueBudgetDao.On("GetByIssueRequestID", mock.Anything, int64(40)).Return(nil, nil)
	rewardRequestDao.On(
		"UpdateStatus", mock.Anything, mock.Anything, int64(100),
		model.RewardRequestStatusPending, model.RewardRequestStatusCompleted,
	).Return(nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
}

func TestProcessVoucherIssueRequestGetProjectBudgetFailed(t *testing.T) {
	initServiceTestEnv()
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := newIssueRecordTestService(
		new(mocks.RewardRequestDao), new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit", mock.Anything, int64(20),
		util.VoucherTypeCrypto, util.UnitCryptoUSDT,
	).Return(nil, errors.New("db down"))

	err := svc.ProcessVoucherIssueRequest(context.Background(), validRewardRequest())
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInternalError, appErr.Code)
}

func TestProcessVoucherIssueRequestGetClientRefIDFailed(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	mockProcessBudgetExists(projectBudgetDao)
	rewardRequestDao.On("GetByClientRefID", mock.Anything, "ref-1").Return(nil, errors.New("db down"))

	err := svc.ProcessVoucherIssueRequest(context.Background(), validRewardRequest())
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInternalError, appErr.Code)
}

func TestProcessVoucherIssueRequestCreateDuplicateClientRefIDFromDAO(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	mockProcessBudgetExists(projectBudgetDao)
	rewardRequestDao.On("GetByClientRefID", mock.Anything, "ref-1").Return(nil, nil)
	rewardRequestDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.RewardRequest")).
		Return(dao.ErrDuplicateClientRefID)

	err := svc.ProcessVoucherIssueRequest(context.Background(), validRewardRequest())
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeDuplicateClientRefID, appErr.Code)
}

func TestProcessVoucherIssueRequestCreateFailed(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	svc := newIssueRecordTestService(
		rewardRequestDao, new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	mockProcessBudgetExists(projectBudgetDao)
	rewardRequestDao.On("GetByClientRefID", mock.Anything, "ref-1").Return(nil, nil)
	rewardRequestDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.RewardRequest")).
		Return(errors.New("db down"))

	err := svc.ProcessVoucherIssueRequest(context.Background(), validRewardRequest())
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInternalError, appErr.Code)
}

func TestExecuteRewardDistributionLoadProjectBudgetFailed(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	riskChecker := new(mockRiskChecker)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		projectBudgetDao, new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		riskChecker, new(mockVoucherIssuer),
	)

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(&model.RewardRequest{
		ID: 100, ClientRefID: "ref-load-pb", ProjectID: 20,
		VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT,
		Amount: "1.0", Status: model.RewardRequestStatusPending,
	}, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-load-pb").Return(nil, nil)
	riskChecker.On("Check", mock.Anything, mock.Anything).Return(true, "")
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit", mock.Anything, int64(20),
		util.VoucherTypeCrypto, util.UnitCryptoUSDT,
	).Return(nil, errors.New("db down"))

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.Error(t, err)
}

func TestExecuteRewardDistributionLoadIssueBudgetFailed(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
	riskChecker := new(mockRiskChecker)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		projectBudgetDao, issueBudgetDao,
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		riskChecker, new(mockVoucherIssuer),
	)

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(&model.RewardRequest{
		ID: 100, ClientRefID: "ref-load-ib", ProjectID: 20,
		VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT,
		Amount: "1.0", Status: model.RewardRequestStatusPending,
	}, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-load-ib").Return(nil, nil)
	riskChecker.On("Check", mock.Anything, mock.Anything).Return(true, "")
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit", mock.Anything, int64(20),
		util.VoucherTypeCrypto, util.UnitCryptoUSDT,
	).Return(&model.ProjectBudget{ID: 50}, nil)
	issueBudgetDao.On(
		"GetFirstAvailableForDistribution", mock.Anything, int64(20),
		util.VoucherTypeCrypto, util.UnitCryptoUSDT, "1.0",
	).Return(nil, errors.New("db down"))

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.Error(t, err)
}

func TestExecuteRewardDistributionRiskFailureEmptyReason(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	resultProducer := new(mockRewardResultProducer)
	riskChecker := new(mockRiskChecker)
	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), resultProducer,
		riskChecker, new(mockVoucherIssuer),
	)

	rewardRequest := &model.RewardRequest{
		ID: 100, ClientRefID: "ref-risk-empty", UserID: 10, ProjectID: 20,
		VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT,
		Amount: "1.0", Status: model.RewardRequestStatusPending,
	}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(rewardRequest, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-risk-empty").Return(nil, nil)
	riskChecker.On("Check", mock.Anything, int64(10)).Return(false, "")
	issueRecordDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	rewardRequestDao.On(
		"UpdateStatus", mock.Anything, mock.Anything, int64(100),
		model.RewardRequestStatusPending, model.RewardRequestStatusCompleted,
	).Return(nil)
	resultProducer.On("PublishResult", mock.Anything, mock.MatchedBy(func(event producer.RewardDistributionResultEvent) bool {
		return event.FailedReason == "risk check failed"
	})).Return(nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
}

func TestExecuteRewardDistributionBusinessFailureEmptyReason(t *testing.T) {
	initServiceTestEnv()
	rewardRequestDao := new(mocks.RewardRequestDao)
	issueRecordDao := new(mocks.IssueRecordDao)
	projectBudgetDao := new(mocks.ProjectBudgetDao)
	issueBudgetDao := new(mocks.IssueBudgetDao)
	resultProducer := new(mockRewardResultProducer)
	riskChecker := new(mockRiskChecker)
	voucherIssuer := new(mockVoucherIssuer)

	svc := newIssueRecordTestService(
		rewardRequestDao, issueRecordDao, new(mocks.ProjectDao),
		projectBudgetDao, issueBudgetDao,
		new(mockRewardExecuteProducer), resultProducer,
		riskChecker, voucherIssuer,
	)

	rewardRequest := &model.RewardRequest{
		ID: 100, ClientRefID: "ref-biz-empty", UserID: 10, ProjectID: 20,
		VoucherType: util.VoucherTypeCrypto, Unit: util.UnitCryptoUSDT,
		Amount: "1.0", Status: model.RewardRequestStatusPending,
	}
	issueBudget := &model.IssueBudget{ID: 30, IssueRequestID: 40, AvailableAmount: "10.0"}
	projectBudget := &model.ProjectBudget{ID: 50, AvailableAmount: "10.0"}

	rewardRequestDao.On("GetByID", mock.Anything, int64(100)).Return(rewardRequest, nil)
	issueRecordDao.On("GetByClientRefId", mock.Anything, "ref-biz-empty").Return(nil, nil).Once()
	riskChecker.On("Check", mock.Anything, int64(10)).Return(true, "")
	issueBudgetDao.On(
		"GetFirstAvailableForDistribution", mock.Anything, int64(20),
		util.VoucherTypeCrypto, util.UnitCryptoUSDT, "1.0",
	).Return(issueBudget, nil).Once()
	projectBudgetDao.On(
		"GetByProjectIDVoucherTypeUnit", mock.Anything, int64(20),
		util.VoucherTypeCrypto, util.UnitCryptoUSDT,
	).Return(projectBudget, nil).Once()
	projectBudgetDao.On("ApplyDistributionDeductAvailable", mock.Anything, mock.Anything, int64(50), "1.0").Return(nil)
	issueBudgetDao.On("ApplyDistributionDeductAvailable", mock.Anything, mock.Anything, int64(30), "1.0").Return(nil)
	issueRecordDao.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	voucherIssuer.On("Issue", mock.Anything, mock.Anything, "1.0").Return(false, "", nil)
	projectBudgetDao.On("ApplyDistributionRefund", mock.Anything, mock.Anything, int64(50), "1.0").Return(nil)
	issueBudgetDao.On("ApplyDistributionRefund", mock.Anything, mock.Anything, int64(30), "1.0").Return(nil)
	issueRecordDao.On("Save", mock.Anything, mock.Anything, mock.AnythingOfType("*model.IssueRecord")).Return(nil)
	rewardRequestDao.On(
		"UpdateStatus", mock.Anything, mock.Anything, int64(100),
		model.RewardRequestStatusPending, model.RewardRequestStatusCompleted,
	).Return(nil)
	resultProducer.On("PublishResult", mock.Anything, mock.MatchedBy(func(event producer.RewardDistributionResultEvent) bool {
		return event.Status == distributionResultStatusFailed &&
			event.FailedReason == "downstream business failure"
	})).Return(nil)

	err := svc.ExecuteRewardDistribution(context.Background(), 100)
	assert.NoError(t, err)
}

func TestListIssueRecordsByProjectAndUserSuccess(t *testing.T) {
	initServiceTestEnv()
	issueRecordDao := new(mocks.IssueRecordDao)
	svc := newIssueRecordTestService(
		new(mocks.RewardRequestDao), issueRecordDao, new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	createdAt := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	issueRecordDao.On("ListByProjectIDAndUserID", mock.Anything, int64(20), int64(10)).Return([]*model.IssueRecord{
		{
			VoucherID:    "v-1",
			VoucherType:  util.VoucherTypeCrypto,
			Unit:         util.UnitCryptoUSDT,
			RewardAmount: "1.5",
			IssueStatus:  model.IssueRecordStatusIssued,
			CreatedAt:    createdAt,
		},
	}, nil).Once()

	items, err := svc.ListIssueRecordsByProjectAndUser(context.Background(), 20, 10)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "v-1", items[0].VoucherID)
	assert.Equal(t, "1.5", items[0].RewardAmount)
	assert.Equal(t, model.IssueRecordStatusIssued, items[0].Status)
	assert.Equal(t, util.FormatDateTime(createdAt), items[0].CreatedAt)
}

func TestListIssueRecordsByProjectAndUserValidation(t *testing.T) {
	initServiceTestEnv()
	svc := newIssueRecordTestService(
		new(mocks.RewardRequestDao), new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	_, err := svc.ListIssueRecordsByProjectAndUser(context.Background(), 0, 10)
	assert.Error(t, err)
	_, err = svc.ListIssueRecordsByProjectAndUser(context.Background(), 20, 0)
	assert.Error(t, err)
}

func TestListIssueRecordsByProjectAndUserDAOError(t *testing.T) {
	initServiceTestEnv()
	issueRecordDao := new(mocks.IssueRecordDao)
	svc := newIssueRecordTestService(
		new(mocks.RewardRequestDao), issueRecordDao, new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)

	issueRecordDao.On("ListByProjectIDAndUserID", mock.Anything, int64(20), int64(10)).
		Return(nil, assert.AnError).Once()

	_, err := svc.ListIssueRecordsByProjectAndUser(context.Background(), 20, 10)
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInternalError, appErr.Code)
}

func TestProcessVoucherIssueRequestTemplateNotFound(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newIssueRecordTestService(
		new(mocks.RewardRequestDao), new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)
	svc.templateDao = templateDao
	templateDao.On("GetByID", mock.Anything, int64(7)).Return(nil, nil).Once()

	err := svc.ProcessVoucherIssueRequest(context.Background(), &rewardpb.RewardDistributionRequest{
		ClientRefId: "ref-1", UserId: 10, ProjectId: 20, TemplateId: 7,
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeTemplateNotFound, appErr.Code)
}

func TestProcessVoucherIssueRequestTemplateNotPublished(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newIssueRecordTestService(
		new(mocks.RewardRequestDao), new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)
	svc.templateDao = templateDao
	templateDao.On("GetByID", mock.Anything, int64(7)).Return(&model.Template{
		ID: 7, Status: model.TemplateStatusDraft,
	}, nil).Once()

	err := svc.ProcessVoucherIssueRequest(context.Background(), &rewardpb.RewardDistributionRequest{
		ClientRefId: "ref-1", UserId: 10, ProjectId: 20, TemplateId: 7,
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.CodeInvalidRequest, appErr.Code)
}

func TestProcessVoucherIssueRequestTemplateLoadError(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newIssueRecordTestService(
		new(mocks.RewardRequestDao), new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)
	svc.templateDao = templateDao
	templateDao.On("GetByID", mock.Anything, int64(7)).Return(nil, assert.AnError).Once()

	err := svc.ProcessVoucherIssueRequest(context.Background(), &rewardpb.RewardDistributionRequest{
		ClientRefId: "ref-1", UserId: 10, ProjectId: 20, TemplateId: 7,
	})
	assert.Error(t, err)
}

func TestProcessVoucherIssueRequestInvalidTemplateConfig(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newIssueRecordTestService(
		new(mocks.RewardRequestDao), new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)
	svc.templateDao = templateDao
	templateDao.On("GetByID", mock.Anything, int64(7)).Return(&model.Template{
		ID: 7, Type: model.TemplateTypeFixed, Config: []byte("{"), Status: model.TemplateStatusPublished,
	}, nil).Once()

	err := svc.ProcessVoucherIssueRequest(context.Background(), &rewardpb.RewardDistributionRequest{
		ClientRefId: "ref-1", UserId: 10, ProjectId: 20, TemplateId: 7,
	})
	assert.Error(t, err)
	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, errs.MsgInvalidTemplateConfig, appErr.Message)
}

func TestProcessVoucherIssueRequestMissingMetric(t *testing.T) {
	initServiceTestEnv()
	templateDao := new(mocks.TemplateDao)
	svc := newIssueRecordTestService(
		new(mocks.RewardRequestDao), new(mocks.IssueRecordDao), new(mocks.ProjectDao),
		new(mocks.ProjectBudgetDao), new(mocks.IssueBudgetDao),
		new(mockRewardExecuteProducer), new(mockRewardResultProducer),
		new(mockRiskChecker), new(mockVoucherIssuer),
	)
	svc.templateDao = templateDao
	dynamicConfig, _ := json.Marshal(model.DynamicTemplateConfig{BaseMetric: "net_deposit", Rate: 0.1})
	templateDao.On("GetByID", mock.Anything, int64(8)).Return(&model.Template{
		ID: 8, Type: model.TemplateTypeDynamic, Config: dynamicConfig, Status: model.TemplateStatusPublished,
	}, nil).Once()

	err := svc.ProcessVoucherIssueRequest(context.Background(), &rewardpb.RewardDistributionRequest{
		ClientRefId: "ref-1", UserId: 10, ProjectId: 20, TemplateId: 8,
	})
	assert.Error(t, err)
}

func TestCalculateTemplateRewardAmountBranches(t *testing.T) {
	initServiceTestEnv()

	_, err := calculateFixedTemplateRewardAmount([]byte(`{"amount":"bad"}`))
	assert.Error(t, err)

	_, err = calculateTemplateRewardAmount(&model.Template{Type: "UNKNOWN"}, nil)
	assert.Error(t, err)

	_, err = parseAndValidateDynamicTemplateConfig([]byte(`{"base_metric":"","rate":0}`))
	assert.Error(t, err)

	_, err = parseRewardMetricValue("net_deposit", map[string]string{"net_deposit": "x"})
	assert.Error(t, err)

	dynamicConfig, _ := json.Marshal(model.DynamicTemplateConfig{
		BaseMetric: "net_deposit", Rate: 0.1, Cap: "bad",
	})
	_, err = calculateDynamicTemplateRewardAmount(dynamicConfig, map[string]string{"net_deposit": "10"})
	assert.Error(t, err)

	underCapConfig, _ := json.Marshal(model.DynamicTemplateConfig{
		BaseMetric: "net_deposit", Rate: 0.1, Cap: "100",
	})
	amount, err := calculateDynamicTemplateRewardAmount(underCapConfig, map[string]string{"net_deposit": "10"})
	assert.NoError(t, err)
	assert.Equal(t, "1", amount.FloatString(0))

	noCapConfig, _ := json.Marshal(model.DynamicTemplateConfig{
		BaseMetric: "net_deposit", Rate: 0.2,
	})
	amount, err = calculateDynamicTemplateRewardAmount(noCapConfig, map[string]string{"net_deposit": "10"})
	assert.NoError(t, err)
	assert.Equal(t, "2", amount.FloatString(0))
}
