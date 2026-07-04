package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/kafka/producer"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"
	"gorm.io/gorm"
)

type IssueRequestService interface {
	CreateIssueRequest(ctx context.Context, docID string, req *data.CreateIssueRequestRequest) (*data.IssueRequestVO, error)
	UpdateIssueRequest(ctx context.Context, docID string, issueRequestID int64, req *data.UpdateIssueRequestRequest) (*data.IssueRequestVO, error)
	SubmitIssueRequest(ctx context.Context, docID string, issueRequestID int64, req *data.SubmitIssueRequestRequest) (*data.UpdateIssueRequestResponse, error)
	ApproveIssueRequest(ctx context.Context, docID string, issueRequestID int64, req *data.ApproveIssueRequestRequest) (*data.UpdateIssueRequestResponse, error)
	ListIssueRequestsByDocID(ctx context.Context, docID string, page, size int) (*data.PageResult, error)
	ProcessKafkaEvent(ctx context.Context, issueRequestID int64) error
}

type IssueRequestServiceImpl struct {
	financeDocDao    dao.FinanceDocDao
	projectBudgetDao dao.ProjectBudgetDao
	issueRequestDao  dao.IssueRequestDao
	issueBudgetDao   dao.IssueBudgetDao
	eventProducer    producer.IssueRequestUpdatedProducer
	txBeginner       repository.TxBeginner
}

var (
	issueRequestServiceOnce sync.Once
	issueRequestServiceInst IssueRequestService
)

func GetIssueRequestService() IssueRequestService {
	issueRequestServiceOnce.Do(func() {
		issueRequestServiceInst = &IssueRequestServiceImpl{
			financeDocDao:    dao.GetFinanceDocDao(),
			projectBudgetDao: dao.GetProjectBudgetDao(),
			issueRequestDao:  dao.GetIssueRequestDao(),
			issueBudgetDao:   dao.GetIssueBudgetDao(),
			eventProducer:    producer.GetIssueRequestUpdatedProducer(),
			txBeginner:       repository.DB,
		}
	})
	return issueRequestServiceInst
}

func (s *IssueRequestServiceImpl) CreateIssueRequest(
	ctx context.Context,
	docID string,
	req *data.CreateIssueRequestRequest,
) (*data.IssueRequestVO, error) {
	doc, err := s.loadApprovedDoc(ctx, docID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, issueRequestErr(ctx, errs.New(errs.CodeInvalidRequest, "request is required"), "create issue request rejected", "doc_id", docID)
	}

	input, err := parseIssueRequestInput(req.VoucherType, req.Unit, req.Amount, req.ExpenseType)
	if err != nil {
		return nil, issueRequestErr(ctx, err, "create issue request rejected", "doc_id", docID)
	}
	if err := s.ensureAvailableAmount(ctx, doc.ProjectID, input); err != nil {
		return nil, err
	}

	creator := strings.TrimSpace(req.Creator)
	if creator == "" {
		creator = defaultCreator
	}

	request := &model.IssueRequest{
		ProjectID:     doc.ProjectID,
		VoucherType:   input.voucherType,
		Unit:          input.unit,
		Amount:        input.amount,
		RequestStatus: model.IssueRequestStatusDraft,
		ExpenseType:   input.expenseType,
		Creator:       creator,
		Remark:        strings.TrimSpace(req.Remark),
	}
	if err := s.issueRequestDao.Create(ctx, request); err != nil {
		return nil, issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err), "create issue request failed", "doc_id", docID)
	}

	log.WithContext(ctx).Infof("issue request created: doc_id=%s issue_request_id=%d", docID, request.ID)
	return toIssueRequestVO(request), nil
}

func (s *IssueRequestServiceImpl) UpdateIssueRequest(
	ctx context.Context,
	docID string,
	issueRequestID int64,
	req *data.UpdateIssueRequestRequest,
) (*data.IssueRequestVO, error) {
	doc, request, err := s.loadEditableIssueRequest(ctx, docID, issueRequestID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, issueRequestErr(ctx, errs.New(errs.CodeInvalidRequest, "request is required"),
			"update issue request rejected", "doc_id", docID, "issue_request_id", issueRequestID)
	}

	input, err := parseIssueRequestUpdate(req.VoucherType, req.Unit, req.Amount)
	if err != nil {
		return nil, issueRequestErr(ctx, err, "update issue request rejected", "doc_id", docID, "issue_request_id", issueRequestID)
	}
	if err := s.ensureAvailableAmount(ctx, doc.ProjectID, input); err != nil {
		return nil, err
	}

	remark := strings.TrimSpace(req.Remark)
	if err := s.issueRequestDao.UpdateFields(ctx, request.ID, input.voucherType, input.unit, input.amount, remark); err != nil {
		return nil, issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			"update issue request failed", "doc_id", docID, "issue_request_id", issueRequestID)
	}

	request.VoucherType = input.voucherType
	request.Unit = input.unit
	request.Amount = input.amount
	request.Remark = remark
	return toIssueRequestVO(request), nil
}

func (s *IssueRequestServiceImpl) SubmitIssueRequest(
	ctx context.Context,
	docID string,
	issueRequestID int64,
	req *data.SubmitIssueRequestRequest,
) (*data.UpdateIssueRequestResponse, error) {
	doc, request, err := s.loadIssueRequestForDoc(ctx, docID, issueRequestID)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.ToUpper(strings.TrimSpace(req.Status)) != model.IssueRequestStatusToApprove {
		return nil, issueRequestErr(ctx, errs.New(errs.CodeInvalidRequest, "status must be TO_APPROVE"),
			"submit issue request rejected", "doc_id", docID, "issue_request_id", issueRequestID)
	}
	if err := validateIssueRequestTransition(request.RequestStatus, model.IssueRequestStatusToApprove); err != nil {
		return nil, err
	}
	if err := s.ensureAvailableAmount(ctx, doc.ProjectID, &issueRequestInput{voucherType: request.VoucherType, unit: request.Unit, amount: request.Amount}); err != nil {
		return nil, err
	}

	remark := strings.TrimSpace(req.Remark)
	err = s.txBeginner.Transaction(func(tx *gorm.DB) error {
		return s.submitIssueRequestInTx(ctx, tx, request.ID, remark)
	})
	if err != nil {
		var appErr *errs.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			"submit issue request failed", "doc_id", docID, "issue_request_id", issueRequestID)
	}

	return &data.UpdateIssueRequestResponse{
		ID:            request.ID,
		RequestStatus: model.IssueRequestStatusToApprove,
		Remark:        remark,
	}, nil
}

func (s *IssueRequestServiceImpl) ApproveIssueRequest(
	ctx context.Context,
	docID string,
	issueRequestID int64,
	req *data.ApproveIssueRequestRequest,
) (*data.UpdateIssueRequestResponse, error) {
	_, request, err := s.loadIssueRequestForDoc(ctx, docID, issueRequestID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, issueRequestErr(ctx, errs.New(errs.CodeInvalidRequest, "request is required"),
			"approve issue request rejected", "doc_id", docID, "issue_request_id", issueRequestID)
	}

	targetStatus := strings.ToUpper(strings.TrimSpace(req.Status))
	if targetStatus != model.IssueRequestStatusApproved && targetStatus != model.IssueRequestStatusRejected {
		return nil, issueRequestErr(ctx, errs.New(errs.CodeInvalidRequest, "unsupported target status"),
			"approve issue request rejected", "doc_id", docID, "issue_request_id", issueRequestID)
	}
	if err := validateIssueRequestTransition(request.RequestStatus, targetStatus); err != nil {
		return nil, err
	}
	if targetStatus == model.IssueRequestStatusApproved {
		if err := s.ensureWithholdAmount(ctx, request); err != nil {
			return nil, err
		}
	}

	remark := strings.TrimSpace(req.Remark)
	if targetStatus == model.IssueRequestStatusApproved {
		if err := s.approveIssueRequestInTx(ctx, request.ID, remark); err != nil {
			return nil, err
		}
		if err := s.eventProducer.PublishIssueRequestUpdated(ctx, request.ID); err != nil {
			return nil, issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err),
				"publish issue request approved event failed", "doc_id", docID, "issue_request_id", issueRequestID)
		}
	} else {
		if err := s.rejectIssueRequestInTx(ctx, request.ID, remark); err != nil {
			return nil, err
		}
	}

	return &data.UpdateIssueRequestResponse{
		ID:            request.ID,
		RequestStatus: targetStatus,
		Remark:        remark,
	}, nil
}

func (s *IssueRequestServiceImpl) ListIssueRequestsByDocID(
	ctx context.Context,
	docID string,
	page, size int,
) (*data.PageResult, error) {
	doc, err := s.loadApprovedDoc(ctx, docID)
	if err != nil {
		return nil, err
	}
	if page <= 0 || size <= 0 {
		return nil, issueRequestErr(ctx, errs.New(errs.CodeInvalidPagination, ""), "list issue requests rejected", "doc_id", docID)
	}

	requests, total, err := s.issueRequestDao.ListByProjectID(ctx, doc.ProjectID, page, size)
	if err != nil {
		return nil, issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			"list issue requests failed", "doc_id", docID)
	}

	items := make([]*data.IssueRequestVO, 0, len(requests))
	for _, request := range requests {
		items = append(items, toIssueRequestVO(request))
	}
	return &data.PageResult{Total: total, Page: page, Size: size, Items: items}, nil
}

func (s *IssueRequestServiceImpl) ProcessKafkaEvent(ctx context.Context, issueRequestID int64) error {
	request, err := s.issueRequestDao.GetByID(ctx, issueRequestID)
	if err != nil {
		return issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			"process issue request event failed", "issue_request_id", issueRequestID)
	}
	if request == nil {
		return issueRequestErr(ctx, errs.New(errs.CodeIssueRequestNotFound, ""),
			"process issue request event rejected", "issue_request_id", issueRequestID)
	}
	if request.RequestStatus == model.IssueRequestStatusOngoing {
		return nil
	}
	if request.RequestStatus != model.IssueRequestStatusApproved {
		log.WithContext(ctx).Infow("skip issue request event for status",
			"issue_request_id", issueRequestID,
			"request_status", request.RequestStatus,
		)
		return nil
	}
	return s.initIssueBudget(ctx, request)
}

func (s *IssueRequestServiceImpl) submitIssueRequestInTx(ctx context.Context, tx *gorm.DB, issueRequestID int64, remark string) error {
	locked, err := s.issueRequestDao.GetByIDForUpdate(ctx, tx, issueRequestID)
	if err != nil {
		return err
	}
	if locked == nil {
		return issueRequestErr(ctx, errs.New(errs.CodeIssueRequestNotFound, ""),
			"submit issue request rejected", "issue_request_id", issueRequestID)
	}
	if err := validateIssueRequestTransition(locked.RequestStatus, model.IssueRequestStatusToApprove); err != nil {
		return err
	}

	budget, err := s.lockProjectBudget(ctx, tx, locked.ProjectID, locked.VoucherType, locked.Unit)
	if err != nil {
		return err
	}
	if err := ensureAvailableGTE(ctx, budget.AvailableAmount, locked.Amount); err != nil {
		return err
	}
	if err := s.issueRequestDao.UpdateStatusInTx(ctx, tx, locked.ID, locked.RequestStatus, model.IssueRequestStatusToApprove, remark); err != nil {
		return err
	}
	return s.projectBudgetDao.ApplySubmitWithhold(ctx, tx, budget.ID, locked.Amount)
}

func (s *IssueRequestServiceImpl) approveIssueRequestInTx(ctx context.Context, issueRequestID int64, remark string) error {
	return s.txBeginner.Transaction(func(tx *gorm.DB) error {
		locked, err := s.issueRequestDao.GetByIDForUpdate(ctx, tx, issueRequestID)
		if err != nil {
			return err
		}
		if locked == nil {
			return issueRequestErr(ctx, errs.New(errs.CodeIssueRequestNotFound, ""),
				"approve issue request rejected", "issue_request_id", issueRequestID)
		}
		if err := validateIssueRequestTransition(locked.RequestStatus, model.IssueRequestStatusApproved); err != nil {
			return err
		}

		budget, err := s.lockProjectBudget(ctx, tx, locked.ProjectID, locked.VoucherType, locked.Unit)
		if err != nil {
			return err
		}
		if err := ensureWithholdGTE(ctx, budget.WithholdAmount, locked.Amount); err != nil {
			return err
		}
		if err := s.issueRequestDao.UpdateStatusInTx(ctx, tx, locked.ID, model.IssueRequestStatusToApprove, model.IssueRequestStatusApproved, remark); err != nil {
			return err
		}
		return s.projectBudgetDao.ApplyApproveIssued(ctx, tx, budget.ID, locked.Amount)
	})
}

func (s *IssueRequestServiceImpl) rejectIssueRequestInTx(ctx context.Context, issueRequestID int64, remark string) error {
	return s.txBeginner.Transaction(func(tx *gorm.DB) error {
		locked, err := s.issueRequestDao.GetByIDForUpdate(ctx, tx, issueRequestID)
		if err != nil {
			return err
		}
		if locked == nil {
			return issueRequestErr(ctx, errs.New(errs.CodeIssueRequestNotFound, ""),
				"reject issue request rejected", "issue_request_id", issueRequestID)
		}
		if err := validateIssueRequestTransition(locked.RequestStatus, model.IssueRequestStatusRejected); err != nil {
			return err
		}

		budget, err := s.lockProjectBudget(ctx, tx, locked.ProjectID, locked.VoucherType, locked.Unit)
		if err != nil {
			return err
		}
		if err := ensureWithholdGTE(ctx, budget.WithholdAmount, locked.Amount); err != nil {
			return err
		}
		if err := s.issueRequestDao.UpdateStatusInTx(ctx, tx, locked.ID, model.IssueRequestStatusToApprove, model.IssueRequestStatusRejected, remark); err != nil {
			return err
		}
		return s.projectBudgetDao.ApplyRejectRelease(ctx, tx, budget.ID, locked.Amount)
	})
}

func (s *IssueRequestServiceImpl) initIssueBudget(ctx context.Context, request *model.IssueRequest) error {
	return s.txBeginner.Transaction(func(tx *gorm.DB) error {
		locked, err := s.issueRequestDao.GetByIDForUpdate(ctx, tx, request.ID)
		if err != nil {
			return err
		}
		if locked == nil || locked.RequestStatus != model.IssueRequestStatusApproved {
			return nil
		}

		issueBudget := &model.IssueBudget{
			IssueRequestID:  locked.ID,
			VoucherType:     locked.VoucherType,
			Unit:            locked.Unit,
			TotalAmount:     locked.Amount,
			AvailableAmount: locked.Amount,
			IssuedAmount:    "0",
			Status:          model.IssueBudgetStatusOngoing,
		}
		if err := s.issueBudgetDao.Create(ctx, tx, issueBudget); err != nil {
			if errors.Is(err, dao.ErrIssueBudgetAlreadyExists) {
				return s.issueRequestDao.MarkOngoing(ctx, tx, locked.ID)
			}
			return err
		}
		return s.issueRequestDao.MarkOngoing(ctx, tx, locked.ID)
	})
}

type issueRequestInput struct {
	voucherType string
	unit        string
	amount      string
	expenseType string
}

func parseIssueRequestInput(voucherType, unit, amount, expenseType string) (*issueRequestInput, error) {
	parsed, err := parseIssueRequestUpdate(voucherType, unit, amount)
	if err != nil {
		return nil, err
	}
	expenseType = strings.ToUpper(strings.TrimSpace(expenseType))
	switch expenseType {
	case model.IssueRequestExpenseTypeRefund, model.IssueRequestExpenseTypeReward, model.IssueRequestExpenseTypeOthers:
	default:
		return nil, errs.New(errs.CodeInvalidRequest, "invalid expense_type")
	}
	parsed.expenseType = expenseType
	return parsed, nil
}

func parseIssueRequestUpdate(voucherType, unit, amount string) (*issueRequestInput, error) {
	voucherType = strings.TrimSpace(voucherType)
	unit = strings.TrimSpace(unit)
	amount = strings.TrimSpace(amount)
	if voucherType == "" || unit == "" || amount == "" {
		return nil, errs.New(errs.CodeInvalidRequest, "voucher_type, unit and amount are required")
	}
	if _, err := util.ParseAmount(amount); err != nil {
		return nil, errs.New(errs.CodeInvalidRequest, "invalid amount")
	}
	return &issueRequestInput{voucherType: voucherType, unit: unit, amount: amount}, nil
}

func (s *IssueRequestServiceImpl) loadApprovedDoc(ctx context.Context, docID string) (*model.FinanceDoc, error) {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return nil, issueRequestErr(ctx, errs.New(errs.CodeInvalidRequest, "doc_id is required"), "issue request rejected")
	}
	doc, err := s.financeDocDao.GetByDocID(ctx, docID)
	if err != nil {
		return nil, issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err), "load finance doc failed", "doc_id", docID)
	}
	if doc == nil {
		return nil, issueRequestErr(ctx, errs.New(errs.CodeFinanceDocNotFound, ""), "issue request rejected", "doc_id", docID)
	}
	if doc.Status != model.FinanceDocStatusApproved {
		return nil, issueRequestErr(ctx, errs.New(errs.CodeFinanceDocNotApproved, ""), "issue request rejected", "doc_id", docID)
	}
	return doc, nil
}

func (s *IssueRequestServiceImpl) loadIssueRequestForDoc(
	ctx context.Context,
	docID string,
	issueRequestID int64,
) (*model.FinanceDoc, *model.IssueRequest, error) {
	doc, err := s.loadApprovedDoc(ctx, docID)
	if err != nil {
		return nil, nil, err
	}
	request, err := s.issueRequestDao.GetByID(ctx, issueRequestID)
	if err != nil {
		return nil, nil, issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			"load issue request failed", "doc_id", docID, "issue_request_id", issueRequestID)
	}
	if request == nil || request.ProjectID != doc.ProjectID {
		return nil, nil, issueRequestErr(ctx, errs.New(errs.CodeIssueRequestNotFound, ""),
			"issue request rejected", "doc_id", docID, "issue_request_id", issueRequestID)
	}
	return doc, request, nil
}

func (s *IssueRequestServiceImpl) loadEditableIssueRequest(
	ctx context.Context,
	docID string,
	issueRequestID int64,
) (*model.FinanceDoc, *model.IssueRequest, error) {
	doc, request, err := s.loadIssueRequestForDoc(ctx, docID, issueRequestID)
	if err != nil {
		return nil, nil, err
	}
	if request.RequestStatus != model.IssueRequestStatusDraft &&
		request.RequestStatus != model.IssueRequestStatusRejected {
		return nil, nil, issueRequestErr(ctx, errs.New(errs.CodeInvalidStatusTransition, "issue request is not editable"),
			"update issue request rejected", "doc_id", docID, "issue_request_id", issueRequestID, "status", request.RequestStatus)
	}
	return doc, request, nil
}

func (s *IssueRequestServiceImpl) ensureAvailableAmount(
	ctx context.Context,
	projectID int64,
	input *issueRequestInput,
) error {
	budget, err := s.projectBudgetDao.GetByProjectIDVoucherTypeUnit(ctx, projectID, input.voucherType, input.unit)
	if err != nil {
		return issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err), "load project budget failed",
			"project_id", projectID, "voucher_type", input.voucherType, "unit", input.unit)
	}
	if budget == nil {
		return issueRequestErr(ctx, errs.New(errs.CodeProjectBudgetNotFound, ""), "issue request rejected",
			"project_id", projectID, "voucher_type", input.voucherType, "unit", input.unit)
	}
	return ensureAvailableGTE(ctx, budget.AvailableAmount, input.amount)
}

func (s *IssueRequestServiceImpl) ensureWithholdAmount(ctx context.Context, request *model.IssueRequest) error {
	budget, err := s.projectBudgetDao.GetByProjectIDVoucherTypeUnit(ctx, request.ProjectID, request.VoucherType, request.Unit)
	if err != nil {
		return issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err), "load project budget failed",
			"issue_request_id", request.ID)
	}
	if budget == nil {
		return issueRequestErr(ctx, errs.New(errs.CodeProjectBudgetNotFound, ""), "approve issue request rejected",
			"issue_request_id", request.ID)
	}
	return ensureWithholdGTE(ctx, budget.WithholdAmount, request.Amount)
}

func (s *IssueRequestServiceImpl) lockProjectBudget(
	ctx context.Context,
	tx *gorm.DB,
	projectID int64,
	voucherType, unit string,
) (*model.ProjectBudget, error) {
	budget, err := s.projectBudgetDao.GetByProjectIDVoucherTypeUnit(ctx, projectID, voucherType, unit)
	if err != nil {
		return nil, err
	}
	if budget == nil {
		return nil, issueRequestErr(ctx, errs.New(errs.CodeProjectBudgetNotFound, ""), "issue request rejected",
			"project_id", projectID, "voucher_type", voucherType, "unit", unit)
	}
	return s.projectBudgetDao.LockByID(ctx, tx, budget.ID)
}

func ensureAvailableGTE(ctx context.Context, available, amount string) error {
	cmp, err := util.CmpAmount(available, amount)
	if err != nil {
		return issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err), "compare available amount failed")
	}
	if cmp < 0 {
		return issueRequestErr(ctx, errs.New(errs.CodeInsufficientAvailable, ""), "insufficient available amount",
			"available_amount", available, "amount", amount)
	}
	return nil
}

func ensureWithholdGTE(ctx context.Context, withhold, amount string) error {
	cmp, err := util.CmpAmount(withhold, amount)
	if err != nil {
		return issueRequestErr(ctx, errs.Wrap(errs.CodeInternalError, err), "compare withhold amount failed")
	}
	if cmp < 0 {
		return issueRequestErr(ctx, errs.New(errs.CodeInsufficientWithhold, ""), "insufficient withhold amount",
			"withhold_amount", withhold, "amount", amount)
	}
	return nil
}

func validateIssueRequestTransition(currentStatus, targetStatus string) error {
	switch targetStatus {
	case model.IssueRequestStatusToApprove:
		if currentStatus != model.IssueRequestStatusDraft && currentStatus != model.IssueRequestStatusRejected {
			return errs.New(errs.CodeInvalidStatusTransition, "only DRAFT or REJECTED can move to TO_APPROVE")
		}
	case model.IssueRequestStatusApproved, model.IssueRequestStatusRejected:
		if currentStatus != model.IssueRequestStatusToApprove {
			return errs.New(errs.CodeInvalidStatusTransition, "only TO_APPROVE can move to APPROVED or REJECTED")
		}
	default:
		return errs.New(errs.CodeInvalidStatusTransition, "unsupported target status")
	}
	return nil
}

func toIssueRequestVO(request *model.IssueRequest) *data.IssueRequestVO {
	return &data.IssueRequestVO{
		ID:            request.ID,
		ProjectID:     request.ProjectID,
		VoucherType:   request.VoucherType,
		Unit:          request.Unit,
		Amount:        request.Amount,
		RequestStatus: request.RequestStatus,
		ExpenseType:   request.ExpenseType,
		Creator:       request.Creator,
		Remark:        request.Remark,
		CreatedAt:     util.FormatDateTime(request.CreatedAt),
		UpdatedAt:     util.FormatDateTime(request.UpdatedAt),
	}
}

func issueRequestErr(ctx context.Context, err error, msg string, keysAndValues ...any) error {
	log.LogAppError(ctx, err, msg, keysAndValues...)
	return err
}

func ParseIssueRequestID(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errs.New(errs.CodeInvalidRequest, "issue_request_id is required")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errs.New(errs.CodeInvalidRequest, "invalid issue_request_id")
	}
	return id, nil
}
