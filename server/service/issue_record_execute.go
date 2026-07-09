package service

import (
	"context"
	"errors"

	"github.com/nusiss-capstone-project/reward-mservice/server/kafka/producer"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
)

func (s *IssueRecordServiceImpl) ensureCompletedAndSkip(
	ctx context.Context,
	rewardRequest *model.RewardRequest,
	record *model.IssueRecord,
) error {
	if rewardRequest.Status == model.RewardRequestStatusCompleted {
		return nil
	}
	if err := s.rewardRequestDao.UpdateStatus(ctx, nil, rewardRequest.ID, model.RewardRequestStatusCompleted); err != nil {
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
		return s.rewardRequestDao.UpdateStatus(ctx, tx, rewardRequest.ID, model.RewardRequestStatusCompleted)
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

func (s *IssueRecordServiceImpl) ensureDistributionPreconditions(
	ctx context.Context,
	rewardRequest *model.RewardRequest,
) error {
	project, err := s.projectDao.GetByID(ctx, rewardRequest.ProjectID)
	if err != nil {
		return err
	}
	if project == nil {
		log.WithContext(ctx).Infow("project not found, defer distribution",
			"reward_request_id", rewardRequest.ID,
			"project_id", rewardRequest.ProjectID,
		)
		return ErrDistributionDeferred
	}

	hasBudget, err := s.projectBudgetDao.HasAvailableForDistribution(
		ctx,
		rewardRequest.ProjectID,
		rewardRequest.VoucherType,
		rewardRequest.Unit,
		rewardRequest.Amount,
	)
	if err != nil {
		return err
	}
	if !hasBudget {
		log.WithContext(ctx).Infow("project budget insufficient, defer distribution",
			"reward_request_id", rewardRequest.ID,
		)
		return ErrDistributionDeferred
	}

	issueBudget, err := s.issueBudgetDao.GetFirstAvailableForDistribution(
		ctx,
		rewardRequest.ProjectID,
		rewardRequest.VoucherType,
		rewardRequest.Unit,
		rewardRequest.Amount,
	)
	if err != nil {
		log.WithContext(ctx).Errorw("failed to get first available for distribution", "error", err)
		return err
	}
	if issueBudget == nil {
		log.WithContext(ctx).Infow("issue budget insufficient, defer distribution",
			"reward_request_id", rewardRequest.ID,
		)
		return ErrDistributionDeferred
	}
	return nil
}

func (s *IssueRecordServiceImpl) runDistributionAttempt(
	ctx context.Context,
	rewardRequest *model.RewardRequest,
	existingIssueRecord *model.IssueRecord,
) error {
	projectBudget, err := s.projectBudgetDao.GetByProjectIDVoucherTypeUnit(
		ctx,
		rewardRequest.ProjectID,
		rewardRequest.VoucherType,
		rewardRequest.Unit,
	)
	if err != nil {
		return err
	}
	//todo shouldn't happen, return non-retriable error
	if projectBudget == nil {
		log.WithContext(ctx).Errorw("project budget not found, defer distribution",
			"reward_request_id", rewardRequest.ID,
			"project_id", rewardRequest.ProjectID,
			"voucher_type", rewardRequest.VoucherType,
			"unit", rewardRequest.Unit,
		)
		return s.rewardRequestDao.UpdateStatus(ctx, nil, rewardRequest.ID, model.RewardRequestStatusCompleted)
	}
	issueBudget, err := s.issueBudgetDao.GetFirstAvailableForDistribution(
		ctx,
		rewardRequest.ProjectID,
		rewardRequest.VoucherType,
		rewardRequest.Unit,
		rewardRequest.Amount,
	)
	if err != nil {
		return err
	}
	if issueBudget == nil {
		log.WithContext(ctx).Infow("issue budget insufficient, defer distribution",
			"reward_request_id", rewardRequest.ID,
		)
		return nil
	}
	var record *model.IssueRecord
	if existingIssueRecord != nil {
		record = existingIssueRecord
	} else {
		record, err = s.preOccupyBudget(ctx, projectBudget, issueBudget, rewardRequest)
		if err != nil {
			return err
		}
	}

	businessSuccess, failedReason, callErr := s.voucherIssuer.Issue(ctx, &issueRecordSnapshot{
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
	if businessSuccess {
		record.IssueStatus = model.IssueRecordStatusIssued
		record.RewardAmount = amount
		record.Reason = ""
	} else {
		record.IssueStatus = model.IssueRecordStatusFailed
		record.RewardAmount = "0"
		record.Reason = failedReason
		if record.Reason == "" {
			record.Reason = "downstream business failure"
		}
	}

	err := s.txBeginner.Transaction(func(tx *gorm.DB) error {
		if businessSuccess {
			if err := s.projectBudgetDao.ApplyDistributionIssued(ctx, tx, projectBudget.ID, amount); err != nil {
				return err
			}
			if err := s.issueBudgetDao.ApplyDistributionIssued(ctx, tx, issueBudget.ID, amount); err != nil {
				return err
			}
		} else {
			if err := s.projectBudgetDao.ApplyDistributionRefund(ctx, tx, projectBudget.ID, amount); err != nil {
				return err
			}
			if err := s.issueBudgetDao.ApplyDistributionRefund(ctx, tx, issueBudget.ID, amount); err != nil {
				return err
			}
		}

		if err := s.issueRecordDao.Save(ctx, tx, record); err != nil {
			return err
		}
		return s.rewardRequestDao.UpdateStatus(ctx, tx, rewardRequest.ID, model.RewardRequestStatusCompleted)
	})
	if err != nil {
		if errors.Is(err, ErrDistributionRetry) {
			return ErrDistributionRetry
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
