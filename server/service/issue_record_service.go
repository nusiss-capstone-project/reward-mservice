package service

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/nusiss-capstone-project/reward-mservice/common/rewardpb"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/kafka/producer"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/proxy"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"
	"gorm.io/gorm"
)

const (
	distributionResultStatusDistributed = "DISTRIBUTED"
	distributionResultStatusFailed      = "FAILED"
)

var (
	ErrDistributionDeferred = errors.New("distribution deferred")
	ErrDistributionRetry    = errors.New("distribution retry")
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
	templateDao      dao.TemplateDao
	executeProducer  producer.RewardDistributionExecuteProducer
	resultProducer   producer.RewardDistributionResultProducer
	riskChecker      proxy.RiskChecker
	voucherIssuer    proxy.VoucherIssuer
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
			templateDao:      dao.GetTemplateDao(),
			executeProducer:  producer.GetRewardDistributionExecuteProducer(),
			resultProducer:   producer.GetRewardDistributionResultProducer(),
			riskChecker:      proxy.GetRiskChecker(),
			voucherIssuer:    proxy.GetVoucherIssuer(),
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
	if err := s.resolveRewardAmount(ctx, input, request); err != nil {
		return issueRecordErr(ctx, err, errs.LogInputError,
			"client_ref_id", input.clientRefID, "template_id", request.GetTemplateId())
	}

	budget, err := s.projectBudgetDao.GetByProjectIDVoucherTypeUnit(ctx, input.projectID, input.voucherType, input.unit)
	if err != nil {
		log.WithContext(ctx).Errorw("failed to get project budget by project id and voucher type and unit", "project_id", input.projectID, "voucher_type", input.voucherType, "unit", input.unit, "error", err)
		return issueRecordErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			errs.LogOperationFailed, "project_id", input.projectID, "voucher_type", input.voucherType, "unit", input.unit)
	}
	if budget == nil {
		log.WithContext(ctx).Errorw("project budget not found", "project_id", input.projectID, "voucher_type", input.voucherType, "unit", input.unit)
		return issueRecordErr(ctx, errs.New(errs.CodeInvalidRequest, "no budget found"),
			errs.LogOperationFailed, "project_id", input.projectID, "voucher_type", input.voucherType, "unit", input.unit)
	}
	existing, err := s.rewardRequestDao.GetByClientRefID(ctx, input.clientRefID)
	if err != nil {
		log.WithContext(ctx).Errorw("failed to get reward request by client ref id", "client_ref_id", input.clientRefID, "error", err)
		return issueRecordErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			errs.LogOperationFailed, "client_ref_id", input.clientRefID)
	}
	if existing != nil {
		log.WithContext(ctx).Errorw("reward request already exists", "client_ref_id", input.clientRefID)
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
		log.WithContext(ctx).Errorw("failed to get reward request by id", "reward_request_id", rewardRequestID, "error", err)
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
	}

	return s.runDistributionAttempt(ctx, rewardRequest, existingRecord)
}

func (s *IssueRecordServiceImpl) ensureCompletedAndSkip(
	ctx context.Context,
	rewardRequest *model.RewardRequest,
	record *model.IssueRecord,
) error {
	if rewardRequest.Status == model.RewardRequestStatusCompleted {
		return nil
	}
	if err := s.rewardRequestDao.UpdateStatus(
		ctx, nil, rewardRequest.ID,
		model.RewardRequestStatusPending, model.RewardRequestStatusCompleted,
	); err != nil {
		if isRewardRequestAlreadyCompleted(err) {
			return nil
		}
		return err
	}
	return s.publishTerminalResult(ctx, rewardRequest, record)
}

func (s *IssueRecordServiceImpl) tryHandleRiskFailure(
	ctx context.Context,
	rewardRequest *model.RewardRequest,
) (bool, error) {
	passed, reason := s.riskChecker.Check(ctx, rewardRequest.UserID)
	if passed {
		return false, nil
	}
	if reason == "" {
		reason = "risk check failed"
	}

	record := &model.IssueRecord{
		VoucherID:         generateVoucherID(),
		RewardRequestID:   rewardRequest.ID,
		ProjectID:         rewardRequest.ProjectID,
		UserID:            rewardRequest.UserID,
		VoucherType:       rewardRequest.VoucherType,
		Unit:              rewardRequest.Unit,
		RewardAmount:      "0",
		IssueStatus:       model.IssueRecordStatusFailed,
		Reason:            reason,
		ClientReferenceID: rewardRequest.ClientRefID,
	}

	err := s.txBeginner.Transaction(func(tx *gorm.DB) error {
		if err := s.issueRecordDao.Create(ctx, tx, record); err != nil {
			return err
		}
		return s.rewardRequestDao.UpdateStatus(
			ctx, tx, rewardRequest.ID,
			model.RewardRequestStatusPending, model.RewardRequestStatusCompleted,
		)
	})
	if err != nil {
		return true, err
	}

	log.WithContext(ctx).Infow("reward distribution failed at risk check",
		"reward_request_id", rewardRequest.ID,
		"voucher_id", record.VoucherID,
	)
	return true, s.publishTerminalResult(ctx, rewardRequest, record)
}

func (s *IssueRecordServiceImpl) runDistributionAttempt(
	ctx context.Context,
	rewardRequest *model.RewardRequest,
	existingIssueRecord *model.IssueRecord,
) error {
	var projectBudget *model.ProjectBudget
	var issueBudget *model.IssueBudget
	var record *model.IssueRecord
	var err error
	if existingIssueRecord != nil {
		projectBudget, issueBudget, record, err = s.loadExistingDistributionBudgets(ctx, rewardRequest, existingIssueRecord)
		if err != nil {
			return err
		}
		if record == nil {
			return nil
		}
	} else {
		projectBudget, issueBudget, err = s.loadDistributionBudgets(ctx, rewardRequest)
		if err != nil {
			return err
		}
		if projectBudget == nil || issueBudget == nil {
			return nil
		}
		record, err = s.preOccupyBudget(ctx, projectBudget, issueBudget, rewardRequest)
		if err != nil {
			return err
		}
	}

	businessSuccess, failedReason, callErr := s.voucherIssuer.Issue(ctx, &proxy.IssueRecordSnapshot{
		VoucherID:   record.VoucherID,
		UserID:      record.UserID,
		ProjectID:   record.ProjectID,
		VoucherType: record.VoucherType,
		Unit:        record.Unit,
	}, rewardRequest.Amount)
	if callErr != nil {
		log.WithContext(ctx).Errorw("downstream voucher issue call failed",
			"reward_request_id", rewardRequest.ID,
			"voucher_id", record.VoucherID,
			"error", callErr,
		)
		return ErrDistributionRetry
	}

	return s.finalizeDistribution(ctx, rewardRequest, record, issueBudget, projectBudget, businessSuccess, failedReason)
}

func (s *IssueRecordServiceImpl) loadDistributionBudgets(
	ctx context.Context,
	rewardRequest *model.RewardRequest,
) (*model.ProjectBudget, *model.IssueBudget, error) {
	projectBudget, err := s.projectBudgetDao.GetByProjectIDVoucherTypeUnit(
		ctx,
		rewardRequest.ProjectID,
		rewardRequest.VoucherType,
		rewardRequest.Unit,
	)
	if err != nil {
		return nil, nil, err
	}
	if projectBudget == nil {
		log.WithContext(ctx).Errorw("invalid request, project budget not found",
			"reward_request_id", rewardRequest.ID,
			"project_id", rewardRequest.ProjectID,
			"voucher_type", rewardRequest.VoucherType,
			"unit", rewardRequest.Unit,
		)
		return nil, nil, s.handleRewardRequestComplete(ctx, rewardRequest.ID)
	}

	issueBudget, err := s.issueBudgetDao.GetFirstAvailableForDistribution(
		ctx,
		rewardRequest.ProjectID,
		rewardRequest.VoucherType,
		rewardRequest.Unit,
		rewardRequest.Amount,
	)
	if err != nil {
		return nil, nil, err
	}
	if issueBudget == nil {
		log.WithContext(ctx).Infow("issue budget insufficient, defer distribution",
			"reward_request_id", rewardRequest.ID,
		)
		return nil, nil, ErrDistributionDeferred
	}
	return projectBudget, issueBudget, nil
}

func (s *IssueRecordServiceImpl) loadExistingDistributionBudgets(
	ctx context.Context,
	rewardRequest *model.RewardRequest,
	existingIssueRecord *model.IssueRecord,
) (*model.ProjectBudget, *model.IssueBudget, *model.IssueRecord, error) {
	record, err := s.issueRecordDao.GetByID(ctx, existingIssueRecord.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	if record == nil {
		return nil, nil, nil, issueRecordErr(ctx, errs.New(errs.CodeInvalidRequest, "issue record not found"),
			errs.LogInputError, "issue_record_id", existingIssueRecord.ID)
	}
	if record.IssueRequestID == nil {
		return nil, nil, nil, issueRecordErr(ctx, errs.New(errs.CodeInvalidRequest, "issue_request_id is required"),
			errs.LogInputError, "issue_record_id", record.ID)
	}

	projectBudgetRef, err := s.projectBudgetDao.GetByProjectIDVoucherTypeUnit(
		ctx,
		record.ProjectID,
		record.VoucherType,
		record.Unit,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	if projectBudgetRef == nil {
		log.WithContext(ctx).Errorw("invalid request, project budget not found for existing issue record",
			"reward_request_id", rewardRequest.ID,
			"issue_record_id", record.ID,
			"project_id", record.ProjectID,
			"voucher_type", record.VoucherType,
			"unit", record.Unit,
		)
		return nil, nil, nil, s.handleRewardRequestComplete(ctx, rewardRequest.ID)
	}
	projectBudget, err := s.projectBudgetDao.GetByID(ctx, projectBudgetRef.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	if projectBudget == nil {
		return nil, nil, nil, issueRecordErr(ctx, errs.New(errs.CodeInvalidRequest, "project budget not found"),
			errs.LogInputError, "project_budget_id", projectBudgetRef.ID)
	}

	issueBudgetRef, err := s.issueBudgetDao.GetByIssueRequestID(ctx, *record.IssueRequestID)
	if err != nil {
		return nil, nil, nil, err
	}
	if issueBudgetRef == nil {
		log.WithContext(ctx).Errorw("issue budget not found for existing issue record",
			"reward_request_id", rewardRequest.ID,
			"issue_record_id", record.ID,
			"issue_request_id", *record.IssueRequestID,
		)
		return nil, nil, nil, s.handleRewardRequestComplete(ctx, rewardRequest.ID)
	}
	issueBudget, err := s.issueBudgetDao.GetByID(ctx, issueBudgetRef.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	if issueBudget == nil {
		return nil, nil, nil, issueRecordErr(ctx, errs.New(errs.CodeInvalidRequest, "issue budget not found"),
			errs.LogInputError, "issue_budget_id", issueBudgetRef.ID)
	}
	return projectBudget, issueBudget, record, nil
}

func (s *IssueRecordServiceImpl) preOccupyBudget(
	ctx context.Context,
	projectBudget *model.ProjectBudget,
	issueBudget *model.IssueBudget,
	rewardRequest *model.RewardRequest,
) (*model.IssueRecord, error) {
	record := &model.IssueRecord{
		VoucherID:         generateVoucherID(),
		RewardRequestID:   rewardRequest.ID,
		IssueRequestID:    &issueBudget.IssueRequestID,
		ProjectID:         rewardRequest.ProjectID,
		UserID:            rewardRequest.UserID,
		VoucherType:       rewardRequest.VoucherType,
		Unit:              rewardRequest.Unit,
		RewardAmount:      rewardRequest.Amount,
		IssueStatus:       model.IssueRecordStatusPending,
		ClientReferenceID: rewardRequest.ClientRefID,
	}

	err := s.txBeginner.Transaction(func(tx *gorm.DB) error {
		if err := s.projectBudgetDao.ApplyDistributionDeductAvailable(ctx, tx, projectBudget.ID, rewardRequest.Amount); err != nil {
			return err
		}
		if err := s.issueBudgetDao.ApplyDistributionDeductAvailable(ctx, tx, issueBudget.ID, rewardRequest.Amount); err != nil {
			return err
		}
		return s.issueRecordDao.Create(ctx, tx, record)
	})
	return record, err
}

func (s *IssueRecordServiceImpl) applyDistributionIssuedBudgets(
	ctx context.Context,
	tx *gorm.DB,
	projectBudget *model.ProjectBudget,
	issueBudget *model.IssueBudget,
	amount string,
) error {
	if err := s.projectBudgetDao.ApplyDistributionIssued(ctx, tx, projectBudget.ID, amount); err != nil {
		return err
	}
	return s.issueBudgetDao.ApplyDistributionIssued(ctx, tx, issueBudget.ID, amount)
}

func (s *IssueRecordServiceImpl) applyDistributionRefundBudgets(
	ctx context.Context,
	tx *gorm.DB,
	projectBudget *model.ProjectBudget,
	issueBudget *model.IssueBudget,
	amount string,
) error {
	if err := s.projectBudgetDao.ApplyDistributionRefund(ctx, tx, projectBudget.ID, amount); err != nil {
		return err
	}
	return s.issueBudgetDao.ApplyDistributionRefund(ctx, tx, issueBudget.ID, amount)
}

func (s *IssueRecordServiceImpl) applyDistributionOutcomeBudgets(
	ctx context.Context,
	tx *gorm.DB,
	projectBudget *model.ProjectBudget,
	issueBudget *model.IssueBudget,
	amount string,
	businessSuccess bool,
) error {
	if businessSuccess {
		return s.applyDistributionIssuedBudgets(ctx, tx, projectBudget, issueBudget, amount)
	}
	return s.applyDistributionRefundBudgets(ctx, tx, projectBudget, issueBudget, amount)
}

func (s *IssueRecordServiceImpl) finalizeDistribution(
	ctx context.Context,
	rewardRequest *model.RewardRequest,
	record *model.IssueRecord,
	issueBudget *model.IssueBudget,
	projectBudget *model.ProjectBudget,
	businessSuccess bool,
	failedReason string,
) error {
	amount := rewardRequest.Amount
	applyIssueRecordOutcome(record, amount, businessSuccess, failedReason)

	err := s.txBeginner.Transaction(func(tx *gorm.DB) error {
		if err := s.applyDistributionOutcomeBudgets(ctx, tx, projectBudget, issueBudget, amount, businessSuccess); err != nil {
			return err
		}

		if err := s.issueRecordDao.Save(ctx, tx, record); err != nil {
			return err
		}
		return s.rewardRequestDao.UpdateStatus(
			ctx, tx, rewardRequest.ID,
			model.RewardRequestStatusPending, model.RewardRequestStatusCompleted,
		)
	})
	if err != nil {
		if isRewardRequestAlreadyCompleted(err) {
			return nil
		}
		return err
	}

	log.WithContext(ctx).Infow("reward distribution completed",
		"reward_request_id", rewardRequest.ID,
		"voucher_id", record.VoucherID,
		"issue_status", record.IssueStatus,
	)
	return s.publishTerminalResult(ctx, rewardRequest, record)
}

func (s *IssueRecordServiceImpl) publishTerminalResult(
	ctx context.Context,
	rewardRequest *model.RewardRequest,
	record *model.IssueRecord,
) error {
	event := producer.RewardDistributionResultEvent{
		ClientRefID: rewardRequest.ClientRefID,
		VoucherID:   record.VoucherID,
	}
	if record.IssueStatus == model.IssueRecordStatusIssued {
		event.Status = distributionResultStatusDistributed
		event.DistributedAmount = record.RewardAmount
	} else {
		event.Status = distributionResultStatusFailed
		event.DistributedAmount = "0"
		event.FailedReason = record.Reason
	}
	return s.resultProducer.PublishResult(ctx, event)
}

func applyIssueRecordOutcome(record *model.IssueRecord, amount string, businessSuccess bool, failedReason string) {
	if businessSuccess {
		record.IssueStatus = model.IssueRecordStatusIssued
		record.RewardAmount = amount
		record.Reason = ""
		return
	}

	record.IssueStatus = model.IssueRecordStatusFailed
	record.RewardAmount = "0"
	record.Reason = failedReason
	if record.Reason == "" {
		record.Reason = "downstream business failure"
	}
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

	return &rewardDistributionInput{
		clientRefID: clientRefID,
		userID:      int64(request.GetUserId()),
		projectID:   int64(request.GetProjectId()),
	}, nil
}

func (s *IssueRecordServiceImpl) resolveRewardAmount(
	ctx context.Context,
	input *rewardDistributionInput,
	request *rewardpb.RewardDistributionRequest,
) error {
	if request.GetTemplateId() > 0 {
		return s.resolveRewardAmountFromTemplate(ctx, input, int64(request.GetTemplateId()), request.GetMetrics())
	}
	return resolveRewardAmountFromRequest(input, request)
}

func resolveRewardAmountFromRequest(
	input *rewardDistributionInput,
	request *rewardpb.RewardDistributionRequest,
) error {
	voucherType, err := util.ValidateVoucherType(request.GetVoucherType())
	if err != nil {
		return err
	}
	unit, err := util.ValidateUnit(request.GetUnit())
	if err != nil {
		return err
	}
	amount, err := util.ParseAmount(request.GetAmount())
	if err != nil || amount.Sign() <= 0 {
		return errs.New(errs.CodeInvalidRequest, errs.MsgInvalidAmount)
	}
	input.voucherType = voucherType
	input.unit = unit
	input.amount = util.FormatAmount(amount)
	return nil
}

func (s *IssueRecordServiceImpl) resolveRewardAmountFromTemplate(
	ctx context.Context,
	input *rewardDistributionInput,
	templateID int64,
	metrics map[string]string,
) error {
	template, err := s.templateDao.GetByID(ctx, templateID)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err)
	}
	if template == nil {
		return errs.New(errs.CodeTemplateNotFound, "")
	}
	if template.Status != model.TemplateStatusPublished {
		return errs.New(errs.CodeInvalidRequest, "template is not published")
	}

	amount, err := calculateTemplateRewardAmount(template, metrics)
	if err != nil {
		return err
	}
	if amount.Sign() <= 0 {
		return errs.New(errs.CodeInvalidRequest, errs.MsgInvalidAmount)
	}

	input.voucherType = template.VoucherType
	input.unit = template.Unit
	input.amount = util.FormatAmount(amount)
	return nil
}

func calculateTemplateRewardAmount(template *model.Template, metrics map[string]string) (*big.Rat, error) {
	switch template.Type {
	case model.TemplateTypeFixed:
		var cfg model.FixTemplateConfig
		if err := json.Unmarshal(template.Config, &cfg); err != nil {
			return nil, errs.New(errs.CodeInvalidRequest, "invalid template config")
		}
		amount, err := util.ParseAmount(cfg.Amount)
		if err != nil {
			return nil, errs.New(errs.CodeInvalidRequest, errs.MsgInvalidAmount)
		}
		return amount, nil
	case model.TemplateTypeDynamic:
		var cfg model.DynamicTemplateConfig
		if err := json.Unmarshal(template.Config, &cfg); err != nil {
			return nil, errs.New(errs.CodeInvalidRequest, "invalid template config")
		}
		if strings.TrimSpace(cfg.BaseMetric) == "" || cfg.Rate <= 0 {
			return nil, errs.New(errs.CodeInvalidRequest, "invalid template config")
		}
		rawMetric, ok := metrics[cfg.BaseMetric]
		if !ok || strings.TrimSpace(rawMetric) == "" {
			return nil, errs.New(errs.CodeInvalidRequest, "metric is required: "+cfg.BaseMetric)
		}
		metricValue, err := util.ParseAmount(rawMetric)
		if err != nil {
			return nil, errs.New(errs.CodeInvalidRequest, "invalid metric value: "+cfg.BaseMetric)
		}
		amount := new(big.Rat).Mul(metricValue, new(big.Rat).SetFloat64(cfg.Rate))
		if strings.TrimSpace(cfg.Cap) != "" {
			capAmount, err := util.ParseAmount(cfg.Cap)
			if err != nil {
				return nil, errs.New(errs.CodeInvalidRequest, "invalid template cap")
			}
			if amount.Cmp(capAmount) > 0 {
				amount = capAmount
			}
		}
		return amount, nil
	default:
		return nil, errs.New(errs.CodeInvalidRequest, "invalid template type")
	}
}

func generateVoucherID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

func isRewardRequestAlreadyCompleted(err error) bool {
	var appErr *errs.AppError
	return errors.As(err, &appErr) && appErr.Code == errs.CodeInvalidStatusTransition
}

func (s *IssueRecordServiceImpl) handleRewardRequestComplete(ctx context.Context, rewardRequestID int64) error {
	err := s.rewardRequestDao.UpdateStatus(
		ctx, nil, rewardRequestID,
		model.RewardRequestStatusPending, model.RewardRequestStatusCompleted,
	)
	if err != nil && isRewardRequestAlreadyCompleted(err) {
		return nil
	}
	return err
}

func issueRecordErr(ctx context.Context, err error, logMsg string, kv ...any) error {
	log.WithContext(ctx).Errorw(logMsg, append([]any{"error", err}, kv...)...)
	return err
}
