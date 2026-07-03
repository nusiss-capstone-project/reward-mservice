package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"
)

type PaymentConfigService interface {
	ListPaymentConfigs(ctx context.Context) ([]*data.PaymentConfigVO, error)
}

type PaymentConfigServiceImpl struct {
	paymentConfigDao dao.PaymentConfigDao
}

var (
	paymentConfigServiceOnce sync.Once
	paymentConfigServiceInst PaymentConfigService
)

func GetPaymentConfigService() PaymentConfigService {
	paymentConfigServiceOnce.Do(func() {
		paymentConfigServiceInst = &PaymentConfigServiceImpl{
			paymentConfigDao: dao.GetPaymentConfigDao(),
		}
	})
	return paymentConfigServiceInst
}

func (s *PaymentConfigServiceImpl) ListPaymentConfigs(ctx context.Context) ([]*data.PaymentConfigVO, error) {
	logger := log.WithContext(ctx)
	configs, err := s.paymentConfigDao.ListAll(ctx)
	if err != nil {
		logger.Errorf("list payment configs failed: %v", err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}

	items := make([]*data.PaymentConfigVO, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, &data.PaymentConfigVO{
			PayAddress:     cfg.PayAddress,
			VoucherType:    cfg.VoucherType,
			PaymentAccount: cfg.PaymentAccount,
		})
	}
	return items, nil
}

type FinanceDocService interface {
	CreateFinanceDoc(ctx context.Context, req *data.CreateFinanceDocRequest) (string, error)
	ListFinanceDocs(ctx context.Context, page, size int) (*data.PageResult, error)
	GetFinanceDocDetail(ctx context.Context, docID string) (*data.FinanceDocVO, error)
	UpdateFinanceDocStatus(ctx context.Context, docID string, req *data.UpdateFinanceDocRequest) (*data.UpdateFinanceDocResponse, error)
}

type FinanceDocServiceImpl struct {
	financeDocDao    dao.FinanceDocDao
	projectDao       dao.ProjectDao
	paymentConfigDao dao.PaymentConfigDao
}

var (
	financeDocServiceOnce sync.Once
	financeDocServiceInst FinanceDocService
)

func GetFinanceDocService() FinanceDocService {
	financeDocServiceOnce.Do(func() {
		financeDocServiceInst = &FinanceDocServiceImpl{
			financeDocDao:    dao.GetFinanceDocDao(),
			projectDao:       dao.GetProjectDao(),
			paymentConfigDao: dao.GetPaymentConfigDao(),
		}
	})
	return financeDocServiceInst
}

func (s *FinanceDocServiceImpl) CreateFinanceDoc(ctx context.Context, req *data.CreateFinanceDocRequest) (string, error) {
	logger := log.WithContext(ctx)
	if req == nil {
		return "", errs.New(errs.CodeInvalidRequest, "request is required")
	}
	if req.ProjectID <= 0 {
		return "", errs.New(errs.CodeInvalidRequest, "project_id is required")
	}
	if len(req.ApplicationDetail) == 0 {
		return "", errs.New(errs.CodeInvalidRequest, "application_detail is required")
	}

	project, err := s.projectDao.GetByID(ctx, req.ProjectID)
	if err != nil {
		logger.Errorf("validate project failed: %v", err)
		return "", errs.Wrap(errs.CodeInternalError, err)
	}
	if project == nil {
		return "", errs.New(errs.CodeProjectNotFound, "")
	}

	payAddresses := make([]string, 0, len(req.ApplicationDetail))
	for _, item := range req.ApplicationDetail {
		if strings.TrimSpace(item.PayAddress) == "" ||
			strings.TrimSpace(item.Amount) == "" ||
			strings.TrimSpace(item.Unit) == "" {
			return "", errs.New(errs.CodeInvalidRequest, "application_detail fields are required")
		}
		payAddresses = append(payAddresses, strings.TrimSpace(item.PayAddress))
	}

	existing, err := s.paymentConfigDao.FindExistingPayAddresses(ctx, payAddresses)
	if err != nil {
		logger.Errorf("validate pay addresses failed: %v", err)
		return "", errs.Wrap(errs.CodeInternalError, err)
	}
	for _, addr := range payAddresses {
		if _, ok := existing[addr]; !ok {
			return "", errs.New(errs.CodeInvalidPayAddress, "pay address not found: "+addr)
		}
	}

	creator := strings.TrimSpace(req.Creator)
	if creator == "" {
		creator = defaultCreator
	}

	detailJSON, err := json.Marshal(req.ApplicationDetail)
	if err != nil {
		logger.Errorf("marshal application detail failed: %v", err)
		return "", errs.Wrap(errs.CodeInternalError, err)
	}

	docID := generateDocID()
	doc := &model.FinanceDoc{
		DocID:             docID,
		ProjectID:         req.ProjectID,
		Description:       strings.TrimSpace(req.Description),
		ApplicationDetail: detailJSON,
		Creator:           creator,
		Status:            model.FinanceDocStatusDraft,
	}
	if err := s.financeDocDao.Create(ctx, doc); err != nil {
		logger.Errorf("create finance doc failed: %v", err)
		return "", errs.Wrap(errs.CodeInternalError, err)
	}
	logger.Infof("finance doc created: doc_id=%s project_id=%d", docID, req.ProjectID)
	return docID, nil
}

func (s *FinanceDocServiceImpl) ListFinanceDocs(ctx context.Context, page, size int) (*data.PageResult, error) {
	logger := log.WithContext(ctx)
	if page <= 0 || size <= 0 {
		return nil, errs.New(errs.CodeInvalidPagination, "")
	}

	docs, total, err := s.financeDocDao.List(ctx, page, size)
	if err != nil {
		logger.Errorf("list finance docs failed: %v", err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}

	items := make([]*data.FinanceDocVO, 0, len(docs))
	for _, doc := range docs {
		vo, err := s.toFinanceDocVO(ctx, doc)
		if err != nil {
			return nil, err
		}
		items = append(items, vo)
	}

	return &data.PageResult{
		Total: total,
		Page:  page,
		Size:  size,
		Items: items,
	}, nil
}

func (s *FinanceDocServiceImpl) GetFinanceDocDetail(ctx context.Context, docID string) (*data.FinanceDocVO, error) {
	logger := log.WithContext(ctx)
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return nil, errs.New(errs.CodeInvalidRequest, "doc_id is required")
	}

	doc, err := s.financeDocDao.GetByDocID(ctx, docID)
	if err != nil {
		logger.Errorf("get finance doc failed: doc_id=%s err=%v", docID, err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	if doc == nil {
		return nil, errs.New(errs.CodeFinanceDocNotFound, "")
	}
	return s.toFinanceDocVO(ctx, doc)
}

func (s *FinanceDocServiceImpl) UpdateFinanceDocStatus(
	ctx context.Context,
	docID string,
	req *data.UpdateFinanceDocRequest,
) (*data.UpdateFinanceDocResponse, error) {
	logger := log.WithContext(ctx)
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return nil, errs.New(errs.CodeInvalidRequest, "doc_id is required")
	}
	if req == nil || strings.TrimSpace(req.Status) == "" {
		return nil, errs.New(errs.CodeInvalidRequest, "status is required")
	}

	doc, err := s.financeDocDao.GetByDocID(ctx, docID)
	if err != nil {
		logger.Errorf("get finance doc failed: doc_id=%s err=%v", docID, err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	if doc == nil {
		return nil, errs.New(errs.CodeFinanceDocNotFound, "")
	}

	targetStatus := strings.ToUpper(strings.TrimSpace(req.Status))
	if err := validateFinanceDocTransition(doc.Status, targetStatus); err != nil {
		return nil, err
	}

	remark := strings.TrimSpace(req.Remark)
	if err := s.financeDocDao.UpdateStatus(ctx, docID, targetStatus, remark); err != nil {
		logger.Errorf("update finance doc status failed: doc_id=%s err=%v", docID, err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	logger.Infof("finance doc status updated: doc_id=%s status=%s", docID, targetStatus)
	return &data.UpdateFinanceDocResponse{
		DocID:  docID,
		Status: targetStatus,
		Remark: remark,
	}, nil
}

func validateFinanceDocTransition(currentStatus, targetStatus string) error {
	switch targetStatus {
	case model.FinanceDocStatusToApprove:
		if currentStatus != model.FinanceDocStatusDraft && currentStatus != model.FinanceDocStatusRejected {
			return errs.New(errs.CodeInvalidStatusTransition, "only DRAFT or REJECTED can move to TO_APPROVE")
		}
	case model.FinanceDocStatusApproved, model.FinanceDocStatusRejected:
		if currentStatus != model.FinanceDocStatusToApprove {
			return errs.New(errs.CodeInvalidStatusTransition, "only TO_APPROVE can move to APPROVED or REJECTED")
		}
	default:
		return errs.New(errs.CodeInvalidStatusTransition, "unsupported target status")
	}
	return nil
}

func (s *FinanceDocServiceImpl) toFinanceDocVO(
	ctx context.Context,
	doc *model.FinanceDoc,
) (*data.FinanceDocVO, error) {
	logger := log.WithContext(ctx)
	detail := make([]data.ApplicationDetailItemVO, 0)
	if len(doc.ApplicationDetail) > 0 {
		if err := json.Unmarshal(doc.ApplicationDetail, &detail); err != nil {
			logger.Errorf("unmarshal application detail failed: doc_id=%s err=%v", doc.DocID, err)
			return nil, errs.Wrap(errs.CodeInternalError, err)
		}
	}

	projectVO, err := s.loadProjectVO(ctx, doc.ProjectID)
	if err != nil {
		return nil, err
	}

	return &data.FinanceDocVO{
		DocID:             doc.DocID,
		ProjectID:         doc.ProjectID,
		Project:           projectVO,
		Description:       doc.Description,
		ApplicationDetail: detail,
		Creator:           doc.Creator,
		Status:            doc.Status,
		Remark:            doc.Remark,
		CreatedAt:         util.FormatDateTime(doc.CreatedAt),
		UpdatedAt:         util.FormatDateTime(doc.UpdatedAt),
	}, nil
}

func (s *FinanceDocServiceImpl) loadProjectVO(ctx context.Context, projectID int64) (*data.ProjectVO, error) {
	project, err := s.projectDao.GetByID(ctx, projectID)
	if err != nil {
		log.WithContext(ctx).Errorf("load project failed: id=%d err=%v", projectID, err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	if project == nil {
		return nil, errs.New(errs.CodeProjectNotFound, "")
	}
	return toProjectVO(project), nil
}

func generateDocID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
