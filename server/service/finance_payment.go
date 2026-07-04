package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"
	"gorm.io/gorm"
)

type FinancePaymentService interface {
	CreateFinancePayment(ctx context.Context, docID string, req *data.CreateFinancePaymentRequest) (*data.FinancePaymentVO, error)
	GetFinancePayment(ctx context.Context, docID, paymentID string) (*data.FinancePaymentVO, error)
	GetFinancePaymentListByDocID(ctx context.Context, docID string) ([]*data.FinancePaymentVO, error)
}

type FinancePaymentServiceImpl struct {
	financeDocDao     dao.FinanceDocDao
	paymentConfigDao  dao.PaymentConfigDao
	projectBudgetDao  dao.ProjectBudgetDao
	financePaymentDao dao.FinancePaymentDao
	txBeginner        repository.TxBeginner
}

type createPaymentInput struct {
	docID          string
	paymentAddress string
	unit           string
	voucherType    string
	amount         string
}

var (
	financePaymentServiceOnce sync.Once
	financePaymentServiceInst FinancePaymentService
)

func GetFinancePaymentService() FinancePaymentService {
	financePaymentServiceOnce.Do(func() {
		financePaymentServiceInst = &FinancePaymentServiceImpl{
			financeDocDao:     dao.GetFinanceDocDao(),
			paymentConfigDao:  dao.GetPaymentConfigDao(),
			projectBudgetDao:  dao.GetProjectBudgetDao(),
			financePaymentDao: dao.GetFinancePaymentDao(),
			txBeginner:        repository.DB,
		}
	})
	return financePaymentServiceInst
}

func (s *FinancePaymentServiceImpl) CreateFinancePayment(
	ctx context.Context,
	docID string,
	req *data.CreateFinancePaymentRequest,
) (*data.FinancePaymentVO, error) {
	input, err := parseCreatePaymentInput(ctx, docID, req)
	if err != nil {
		return nil, err
	}

	doc, err := s.loadApprovedFinanceDoc(ctx, input.docID)
	if err != nil {
		return nil, err
	}

	if _, err := s.resolvePaymentConfig(ctx, input); err != nil {
		return nil, err
	}

	docAmount, err := resolveDocAmount(ctx, doc, input)
	if err != nil {
		return nil, err
	}

	budget, err := s.loadProjectBudget(ctx, input)
	if err != nil {
		return nil, err
	}

	if err := validatePaymentBudgetAmount(ctx, input.docID, budget, input.amount, docAmount); err != nil {
		return nil, err
	}

	payment := newFinancePayment(input)
	if err := s.createFinancePayment(ctx, payment, budget, docAmount); err != nil {
		return nil, err
	}

	log.WithContext(ctx).Infof(
		"finance payment created: doc_id=%s payment_id=%s amount=%s",
		input.docID, payment.PaymentID, input.amount,
	)
	return toFinancePaymentVO(payment), nil
}

func (s *FinancePaymentServiceImpl) GetFinancePayment(
	ctx context.Context,
	docID, paymentID string,
) (*data.FinancePaymentVO, error) {
	docID = strings.TrimSpace(docID)
	paymentID = strings.TrimSpace(paymentID)
	if docID == "" || paymentID == "" {
		return nil, paymentErr(ctx, errs.New(errs.CodeInvalidRequest, "doc_id and payment_id are required"),
			"get finance payment rejected", "doc_id", docID, "payment_id", paymentID)
	}

	if err := s.ensureFinanceDocExists(ctx, docID, paymentID); err != nil {
		return nil, err
	}

	payment, err := s.financePaymentDao.GetByPaymentIDAndDocID(ctx, paymentID, docID)
	if err != nil {
		return nil, paymentErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			"get finance payment failed", "doc_id", docID, "payment_id", paymentID)
	}
	if payment == nil {
		return nil, paymentErr(ctx, errs.New(errs.CodeFinancePaymentNotFound, ""),
			"get finance payment rejected", "doc_id", docID, "payment_id", paymentID)
	}
	return toFinancePaymentVO(payment), nil
}

func (s *FinancePaymentServiceImpl) GetFinancePaymentListByDocID(
	ctx context.Context,
	docID string,
) ([]*data.FinancePaymentVO, error) {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return nil, paymentErr(ctx, errs.New(errs.CodeInvalidRequest, "doc_id is required"),
			"list finance payments rejected", "doc_id", docID)
	}

	if err := s.ensureFinanceDocExists(ctx, docID, ""); err != nil {
		return nil, err
	}

	payments, err := s.financePaymentDao.ListByDocID(ctx, docID)
	if err != nil {
		return nil, paymentErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			"list finance payments failed", "doc_id", docID)
	}
	return toFinancePaymentVOs(payments), nil
}

func parseCreatePaymentInput(
	ctx context.Context,
	docID string,
	req *data.CreateFinancePaymentRequest,
) (*createPaymentInput, error) {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return nil, paymentErr(ctx, errs.New(errs.CodeInvalidRequest, "doc_id is required"), "create finance payment rejected")
	}
	if req == nil {
		return nil, paymentErr(ctx, errs.New(errs.CodeInvalidRequest, "request is required"),
			"create finance payment rejected", "doc_id", docID)
	}

	input := &createPaymentInput{
		docID:          docID,
		paymentAddress: strings.TrimSpace(req.PaymentAddress),
		unit:           strings.TrimSpace(req.Unit),
		amount:         strings.TrimSpace(req.Amount),
	}
	if input.paymentAddress == "" || input.unit == "" || input.amount == "" {
		return nil, paymentErr(ctx, errs.New(errs.CodeInvalidRequest, "payment_address, amount and unit are required"),
			"create finance payment rejected", "doc_id", docID)
	}
	if _, err := util.ParseAmount(input.amount); err != nil {
		return nil, paymentErr(ctx, errs.New(errs.CodeInvalidRequest, "invalid amount"),
			"create finance payment rejected", "doc_id", docID, "amount", input.amount)
	}
	return input, nil
}

func (s *FinancePaymentServiceImpl) loadApprovedFinanceDoc(ctx context.Context, docID string) (*model.FinanceDoc, error) {
	doc, err := s.financeDocDao.GetByDocID(ctx, docID)
	if err != nil {
		return nil, paymentErr(ctx, errs.Wrap(errs.CodeInternalError, err), "load finance doc failed", "doc_id", docID)
	}
	if doc == nil {
		return nil, paymentErr(ctx, errs.New(errs.CodeFinanceDocNotFound, ""),
			"create finance payment rejected", "doc_id", docID)
	}
	if doc.Status != model.FinanceDocStatusApproved {
		return nil, paymentErr(ctx, errs.New(errs.CodeFinanceDocNotApproved, ""),
			"create finance payment rejected", "doc_id", docID, "status", doc.Status)
	}
	return doc, nil
}

func (s *FinancePaymentServiceImpl) resolvePaymentConfig(
	ctx context.Context,
	input *createPaymentInput,
) (*model.PaymentConfig, error) {
	cfg, err := s.paymentConfigDao.GetByPayAddress(ctx, input.paymentAddress)
	if err != nil {
		return nil, paymentErr(ctx, errs.Wrap(errs.CodeInternalError, err), "load payment config failed",
			"doc_id", input.docID, "payment_address", input.paymentAddress, "unit", input.unit)
	}
	if cfg == nil {
		return nil, paymentErr(ctx, errs.New(errs.CodeInvalidPayAddress, "payment_address and unit do not match payment config"),
			"create finance payment rejected", "doc_id", input.docID, "payment_address", input.paymentAddress, "unit", input.unit)
	}
	input.voucherType = cfg.VoucherType
	input.unit = cfg.Unit
	return cfg, nil
}

func resolveDocAmount(
	ctx context.Context,
	doc *model.FinanceDoc,
	input *createPaymentInput,
) (string, error) {
	docAmount, ok := findApplicationDetailAmount(doc.ApplicationDetail, input)
	if !ok {
		return "", paymentErr(ctx, errs.New(errs.CodeInvalidRequest, "payment_address and unit do not match finance doc application_detail"),
			"create finance payment rejected", "doc_id", input.docID, "payment_address", input.paymentAddress)
	}
	return docAmount, nil
}

func (s *FinancePaymentServiceImpl) loadProjectBudget(
	ctx context.Context,
	input *createPaymentInput,
) (*model.ProjectBudget, error) {
	budget, err := s.projectBudgetDao.GetByDocIDVoucherTypeUnit(ctx, input.docID, input.voucherType, input.unit)
	if err != nil {
		return nil, paymentErr(ctx, errs.Wrap(errs.CodeInternalError, err), "load project budget failed",
			"doc_id", input.docID, "voucher_type", input.voucherType, "unit", input.unit)
	}
	if budget == nil {
		return nil, paymentErr(ctx, errs.New(errs.CodeProjectBudgetNotFound, ""),
			"create finance payment rejected", "doc_id", input.docID, "voucher_type", input.voucherType, "unit", input.unit)
	}
	return budget, nil
}

func validatePaymentBudgetAmount(
	ctx context.Context,
	docID string,
	budget *model.ProjectBudget,
	amount, docAmount string,
) error {
	nextTotal, err := util.AddAmount(budget.TotalAmount, amount)
	if err != nil {
		return paymentErr(ctx, errs.New(errs.CodeInvalidRequest, "invalid amount"),
			"create finance payment rejected", "doc_id", docID, "amount", amount, "budget_total_amount", budget.TotalAmount)
	}
	cmp, err := util.CmpAmount(nextTotal, docAmount)
	if err != nil {
		return paymentErr(ctx, errs.Wrap(errs.CodeInternalError, err), "compare payment amount failed",
			"doc_id", docID, "next_total", nextTotal, "doc_amount", docAmount)
	}
	if cmp > 0 {
		return paymentErr(ctx, errs.New(errs.CodePaymentExceedsDocAmount, ""),
			"create finance payment rejected",
			"doc_id", docID, "budget_total_amount", budget.TotalAmount, "amount", amount, "doc_amount", docAmount)
	}
	return nil
}

func newFinancePayment(input *createPaymentInput) *model.FinancePayment {
	return &model.FinancePayment{
		FinanceDocID:     input.docID,
		PaymentID:        generatePaymentID(),
		PaymentAddress:   input.paymentAddress,
		Amount:           input.amount,
		PaymentStatus:    model.FinancePaymentStatusPaid,
		OffsettedAmount:  "0",
		OffsettingAmount: "0",
		RefundedAmount:   "0",
		RefundingAmount:  "0",
	}
}

func (s *FinancePaymentServiceImpl) createFinancePayment(
	ctx context.Context,
	payment *model.FinancePayment,
	budget *model.ProjectBudget,
	docAmount string,
) error {
	err := s.txBeginner.Transaction(func(tx *gorm.DB) error {
		return s.createFinancePaymentInTx(ctx, tx, payment, budget, docAmount)
	})
	if err != nil {
		var appErr *errs.AppError
		if errors.As(err, &appErr) {
			return err
		}
		return paymentErr(ctx, errs.Wrap(errs.CodeInternalError, err), "create finance payment failed",
			"doc_id", payment.FinanceDocID, "payment_id", payment.PaymentID)
	}
	return nil
}

func (s *FinancePaymentServiceImpl) createFinancePaymentInTx(
	ctx context.Context,
	tx *gorm.DB,
	payment *model.FinancePayment,
	budget *model.ProjectBudget,
	docAmount string,
) error {
	locked, err := s.projectBudgetDao.LockByID(ctx, tx, budget.ID)
	if err != nil {
		return err
	}
	if locked == nil {
		return paymentErr(ctx, errs.New(errs.CodeProjectBudgetNotFound, ""),
			"create finance payment rejected", "doc_id", payment.FinanceDocID, "budget_id", budget.ID)
	}
	if err := validatePaymentBudgetAmount(ctx, payment.FinanceDocID, locked, payment.Amount, docAmount); err != nil {
		return err
	}
	if err := s.financePaymentDao.Create(ctx, tx, payment); err != nil {
		return err
	}
	return s.projectBudgetDao.UpdateTotalAndAvailableAmount(ctx, tx, locked.ID, payment.Amount)
}

func (s *FinancePaymentServiceImpl) ensureFinanceDocExists(ctx context.Context, docID, paymentID string) error {
	doc, err := s.financeDocDao.GetByDocID(ctx, docID)
	if err != nil {
		return paymentErr(ctx, errs.Wrap(errs.CodeInternalError, err), "load finance doc failed", "doc_id", docID)
	}
	if doc == nil {
		return paymentErr(ctx, errs.New(errs.CodeFinanceDocNotFound, ""),
			"get finance payment rejected", "doc_id", docID, "payment_id", paymentID)
	}
	return nil
}

func paymentErr(ctx context.Context, err error, msg string, keysAndValues ...any) error {
	log.LogAppError(ctx, err, msg, keysAndValues...)
	return err
}

func findApplicationDetailAmount(raw []byte, input *createPaymentInput) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	detail := make([]model.ApplicationDetailItem, 0)
	if err := json.Unmarshal(raw, &detail); err != nil {
		return "", false
	}
	for _, item := range detail {
		if strings.TrimSpace(item.PayAddress) == input.paymentAddress {
			amount := strings.TrimSpace(item.Amount)
			if amount == "" {
				return "", false
			}
			return amount, true
		}
	}
	return "", false
}

func toFinancePaymentVO(payment *model.FinancePayment) *data.FinancePaymentVO {
	return &data.FinancePaymentVO{
		FinanceDocID:     payment.FinanceDocID,
		PaymentID:        payment.PaymentID,
		PaymentAddress:   payment.PaymentAddress,
		Amount:           payment.Amount,
		PaymentStatus:    payment.PaymentStatus,
		OffsettedAmount:  payment.OffsettedAmount,
		OffsettingAmount: payment.OffsettingAmount,
		RefundedAmount:   payment.RefundedAmount,
		RefundingAmount:  payment.RefundingAmount,
		CreatedAt:        util.FormatDateTime(payment.CreatedAt),
		UpdatedAt:        util.FormatDateTime(payment.UpdatedAt),
	}
}

func toFinancePaymentVOs(payments []*model.FinancePayment) []*data.FinancePaymentVO {
	result := make([]*data.FinancePaymentVO, 0, len(payments))
	for _, payment := range payments {
		result = append(result, toFinancePaymentVO(payment))
	}
	return result
}

func generatePaymentID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
