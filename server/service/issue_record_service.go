package service

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/nusiss-capstone-project/reward-mservice/common/rewardpb"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/kafka/producer"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"
)

type IssueRecordService interface {
	ProcessVoucherIssueRequest(ctx context.Context, request *rewardpb.RewardDistributionRequest) error
	ExecuteRewardDistribution(ctx context.Context, rewardRequestID int64) error
}

type IssueRecordServiceImpl struct {
	rewardRequestDao dao.RewardRequestDao
	issueRecordDao   dao.IssueRecordDao
	projectDao       dao.ProjectDao
	projectBudgetDao dao.ProjectBudgetDao
	issueBudgetDao   dao.IssueBudgetDao
	executeProducer  producer.RewardDistributionExecuteProducer
	resultProducer   producer.RewardDistributionResultProducer
	riskChecker      RiskChecker
	voucherIssuer    VoucherIssuer
	txBeginner       repository.TxBeginner
}

var (
	issueRecordServiceOnce sync.Once
	issueRecordServiceInst IssueRecordService
)

func GetIssueRecordService() IssueRecordService {
	issueRecordServiceOnce.Do(func() {
		issueRecordServiceInst = &IssueRecordServiceImpl{
			rewardRequestDao: dao.GetRewardRequestDao(),
			issueRecordDao:   dao.GetIssueRecordDao(),
			projectDao:       dao.GetProjectDao(),
			projectBudgetDao: dao.GetProjectBudgetDao(),
			issueBudgetDao:   dao.GetIssueBudgetDao(),
			executeProducer:  producer.GetRewardDistributionExecuteProducer(),
			resultProducer:   producer.GetRewardDistributionResultProducer(),
			riskChecker:      noopRiskChecker{},
			voucherIssuer:    noopVoucherIssuer{},
			txBeginner:       repository.DB,
		}
	})
	return issueRecordServiceInst
}

func (s *IssueRecordServiceImpl) ProcessVoucherIssueRequest(
	ctx context.Context,
	request *rewardpb.RewardDistributionRequest,
) error {
	if request == nil {
		return issueRecordErr(ctx, errs.New(errs.CodeInvalidRequest, errs.MsgRequestRequired),
			errs.LogInputError, "reason", "nil request")
	}

	input, err := parseRewardDistributionRequest(request)
	if err != nil {
		return issueRecordErr(ctx, err, errs.LogInputError, "client_ref_id", request.GetClientRefId())
	}

	existing, err := s.rewardRequestDao.GetByClientRefID(ctx, input.clientRefID)
	if err != nil {
		return issueRecordErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			errs.LogOperationFailed, "client_ref_id", input.clientRefID)
	}
	if existing != nil {
		return issueRecordErr(ctx, errs.New(errs.CodeDuplicateClientRefID, ""),
			errs.LogInputError, "client_ref_id", input.clientRefID)
	}

	rewardRequest := &model.RewardRequest{
		ClientRefID: input.clientRefID,
		UserID:      input.userID,
		ProjectID:   input.projectID,
		VoucherType: input.voucherType,
		Unit:        input.unit,
		Amount:      input.amount,
		Status:      model.RewardRequestStatusPending,
	}
	if err := s.rewardRequestDao.Create(ctx, nil, rewardRequest); err != nil {
		if errors.Is(err, dao.ErrDuplicateClientRefID) {
			return issueRecordErr(ctx, errs.New(errs.CodeDuplicateClientRefID, ""),
				errs.LogInputError, "client_ref_id", input.clientRefID)
		}
		return issueRecordErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			errs.LogOperationFailed, "client_ref_id", input.clientRefID)
	}

	if err := s.executeProducer.PublishExecute(ctx, rewardRequest.ID); err != nil {
		return issueRecordErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			errs.LogOperationFailed, "reward_request_id", rewardRequest.ID)
	}

	log.WithContext(ctx).Infow("reward distribution request accepted",
		"reward_request_id", rewardRequest.ID,
		"client_ref_id", input.clientRefID,
	)
	return nil
}

func (s *IssueRecordServiceImpl) ExecuteRewardDistribution(ctx context.Context, rewardRequestID int64) error {
	if rewardRequestID <= 0 {
		return issueRecordErr(ctx, errs.New(errs.CodeInvalidRequest, "reward_request_id must be positive"),
			errs.LogInputError, "reward_request_id", rewardRequestID)
	}

	rewardRequest, err := s.rewardRequestDao.GetByID(ctx, rewardRequestID)
	if err != nil {
		return err
	}
	if rewardRequest == nil {
		log.WithContext(ctx).Infow("reward request not found, skip", "reward_request_id", rewardRequestID)
		return nil
	}
	if rewardRequest.Status != model.RewardRequestStatusPending {
		return nil
	}

	existingRecord, err := s.issueRecordDao.GetByClientRefId(ctx, rewardRequest.ClientRefID)
	if err != nil {
		log.WithContext(ctx).Errorw("failed to get issue record by client ref id", "client_ref_id", rewardRequest.ClientRefID, "error", err)
		return err
	}
	if existingRecord != nil && existingRecord.IssueStatus != model.IssueRecordStatusPending {
		log.WithContext(ctx).Infow("issue record already exists and is not pending, skip", "client_ref_id", rewardRequest.ClientRefID)
		return s.ensureCompletedAndSkip(ctx, rewardRequest, existingRecord)
	}

	if existingRecord == nil {
		if handled, err := s.tryHandleRiskFailure(ctx, rewardRequest); handled || err != nil {
			return err
		}
		if err := s.ensureDistributionPreconditions(ctx, rewardRequest); err != nil {
			if errors.Is(err, ErrDistributionDeferred) {
				return nil
			}
			return err
		}
	}

	return s.runDistributionAttempt(ctx, rewardRequest, existingRecord)
}

type rewardDistributionInput struct {
	clientRefID string
	userID      int64
	projectID   int64
	voucherType string
	unit        string
	amount      string
}

func parseRewardDistributionRequest(request *rewardpb.RewardDistributionRequest) (*rewardDistributionInput, error) {
	clientRefID := strings.TrimSpace(request.GetClientRefId())
	if clientRefID == "" {
		return nil, errs.New(errs.CodeInvalidRequest, "client_ref_id is required")
	}
	if request.GetUserId() == 0 {
		return nil, errs.New(errs.CodeInvalidRequest, "user_id is required")
	}
	if request.GetProjectId() == 0 {
		return nil, errs.New(errs.CodeInvalidRequest, "project_id is required")
	}

	voucherType, err := validateRewardVoucherType(request.GetVoucherType())
	if err != nil {
		return nil, err
	}
	unit, err := validateRewardUnit(request.GetUnit())
	if err != nil {
		return nil, err
	}

	amount, err := util.ParseAmount(request.GetAmount())
	if err != nil || amount.Sign() <= 0 {
		return nil, errs.New(errs.CodeInvalidRequest, errs.MsgInvalidAmount)
	}

	return &rewardDistributionInput{
		clientRefID: clientRefID,
		userID:      int64(request.GetUserId()),
		projectID:   int64(request.GetProjectId()),
		voucherType: voucherType,
		unit:        unit,
		amount:      util.FormatAmount(amount),
	}, nil
}

func generateVoucherID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

func issueRecordErr(ctx context.Context, err error, logMsg string, kv ...any) error {
	log.WithContext(ctx).Errorw(logMsg, append([]any{"error", err}, kv...)...)
	return err
}
