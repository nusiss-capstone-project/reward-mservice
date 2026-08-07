package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/kafka/producer"
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
			Unit:           cfg.Unit,
			PaymentAccount: cfg.PaymentAccount,
		})
	}
	return items, nil
}

type FinanceDocService interface {
	CreateFinanceDoc(ctx context.Context, req *data.CreateFinanceDocRequest) (string, error)
	ListFinanceDocs(ctx context.Context, page, size int, status string) (*data.PageResult, error)
	GetFinanceDocDetail(ctx context.Context, docID string) (*data.FinanceDocVO, error)
	UpdateFinanceDoc(ctx context.Context, docID string, req *data.UpdateFinanceDocContentRequest) (*data.FinanceDocVO, error)
	UpdateFinanceDocStatus(ctx context.Context, docID string, req *data.UpdateFinanceDocRequest) (*data.UpdateFinanceDocResponse, error)
	ApproveFinanceDoc(ctx context.Context, docID string, req *data.ApproveFinanceDocRequest) (*data.UpdateFinanceDocResponse, error)
}

type FinanceDocServiceImpl struct {
	financeDocDao              dao.FinanceDocDao
	projectDao                 dao.ProjectDao
	paymentConfigDao           dao.PaymentConfigDao
	financeDocApprovedProducer producer.FinanceDocApprovedProducer
}

var (
	financeDocServiceOnce sync.Once
	financeDocServiceInst FinanceDocService
)

func GetFinanceDocService() FinanceDocService {
	financeDocServiceOnce.Do(func() {
		financeDocServiceInst = &FinanceDocServiceImpl{
			financeDocDao:              dao.GetFinanceDocDao(),
			projectDao:                 dao.GetProjectDao(),
			paymentConfigDao:           dao.GetPaymentConfigDao(),
			financeDocApprovedProducer: producer.GetFinanceDocApprovedProducer(),
		}
	})
	return financeDocServiceInst
}

func (s *FinanceDocServiceImpl) CreateFinanceDoc(ctx context.Context, req *data.CreateFinanceDocRequest) (string, error) {
	logger := log.WithContext(ctx)
	if req == nil {
		return "", errs.New(errs.CodeInvalidRequest, errs.MsgRequestRequired)
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

	exists, err := s.financeDocDao.ExistsByProjectID(ctx, req.ProjectID)
	if err != nil {
		logger.Errorf("check finance doc by project failed: %v", err)
		return "", errs.Wrap(errs.CodeInternalError, err)
	}
	if exists {
		return "", errs.New(errs.CodeFinanceDocProjectExists, "")
	}

	detailItems := toApplicationDetailItems(req.ApplicationDetail)
	if _, err := resolveBudgetItems(ctx, s.paymentConfigDao, detailItems); err != nil {
		return "", err
	}

	creator, err := util.CurrentUserIDString(ctx)
	if err != nil {
		return "", err
	}

	detailJSON, err := json.Marshal(detailItems)
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

func (s *FinanceDocServiceImpl) ListFinanceDocs(ctx context.Context, page, size int, status string) (*data.PageResult, error) {
	logger := log.WithContext(ctx)
	if page <= 0 || size <= 0 {
		return nil, errs.New(errs.CodeInvalidPagination, "")
	}

	creator, err := util.ListCreatorFilter(ctx)
	if err != nil {
		return nil, err
	}
	status = strings.TrimSpace(status)

	docs, total, err := s.financeDocDao.List(ctx, page, size, status, creator)
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
		return nil, errs.New(errs.CodeInvalidRequest, errs.MsgDocIDRequired)
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

func (s *FinanceDocServiceImpl) UpdateFinanceDoc(
	ctx context.Context,
	docID string,
	req *data.UpdateFinanceDocContentRequest,
) (*data.FinanceDocVO, error) {
	logger := log.WithContext(ctx)
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return nil, errs.New(errs.CodeInvalidRequest, errs.MsgDocIDRequired)
	}
	if req == nil {
		return nil, errs.New(errs.CodeInvalidRequest, errs.MsgRequestRequired)
	}
	if len(req.ApplicationDetail) == 0 {
		return nil, errs.New(errs.CodeInvalidRequest, "application_detail is required")
	}

	doc, err := s.financeDocDao.GetByDocID(ctx, docID)
	if err != nil {
		logger.Errorf("get finance doc failed: doc_id=%s err=%v", docID, err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	if doc == nil {
		return nil, errs.New(errs.CodeFinanceDocNotFound, "")
	}
	if err := validateFinanceDocEditable(doc.Status); err != nil {
		return nil, err
	}

	detailItems := toApplicationDetailItems(req.ApplicationDetail)
	if _, err := resolveBudgetItems(ctx, s.paymentConfigDao, detailItems); err != nil {
		return nil, err
	}

	detailJSON, err := json.Marshal(detailItems)
	if err != nil {
		logger.Errorf("marshal application detail failed: %v", err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}

	description := strings.TrimSpace(req.Description)
	if err := s.financeDocDao.UpdateContent(ctx, docID, description, detailJSON); err != nil {
		logger.Errorf("update finance doc content failed: doc_id=%s err=%v", docID, err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}

	doc.Description = description
	doc.ApplicationDetail = detailJSON
	logger.Infof("finance doc updated: doc_id=%s", docID)
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
		return nil, errs.New(errs.CodeInvalidRequest, errs.MsgDocIDRequired)
	}
	if req == nil || strings.TrimSpace(req.Status) == "" {
		return nil, errs.New(errs.CodeInvalidRequest, errs.MsgStatusRequired)
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

func (s *FinanceDocServiceImpl) ApproveFinanceDoc(
	ctx context.Context,
	docID string,
	req *data.ApproveFinanceDocRequest,
) (*data.UpdateFinanceDocResponse, error) {
	logger := log.WithContext(ctx)
	if req == nil || strings.TrimSpace(req.Status) == "" {
		return nil, errs.New(errs.CodeInvalidRequest, errs.MsgStatusRequired)
	}

	updateReq := &data.UpdateFinanceDocRequest{
		Status: req.Status,
		Remark: req.Remark,
	}
	resp, err := s.UpdateFinanceDocStatus(ctx, docID, updateReq)
	if err != nil {
		return nil, err
	}

	if resp.Status != model.FinanceDocStatusApproved {
		return resp, nil
	}

	doc, err := s.financeDocDao.GetByDocID(ctx, resp.DocID)
	if err != nil {
		logger.Errorf("load finance doc after approve failed: doc_id=%s err=%v", resp.DocID, err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	if doc == nil {
		return nil, errs.New(errs.CodeFinanceDocNotFound, "")
	}

	if err := s.financeDocApprovedProducer.PublishFinanceDocApproved(ctx, doc.DocID, doc.ProjectID); err != nil {
		logger.Errorf("publish finance doc approved event failed: doc_id=%s err=%v", doc.DocID, err)
		return nil, errs.Wrap(errs.CodeInternalError, err)
	}
	logger.Infof("finance doc approved event published: doc_id=%s project_id=%d", doc.DocID, doc.ProjectID)
	return resp, nil
}

func validateFinanceDocEditable(status string) error {
	if status != model.FinanceDocStatusDraft && status != model.FinanceDocStatusRejected {
		return errs.New(errs.CodeInvalidStatusTransition, "finance doc is not editable")
	}
	return nil
}

func validateFinanceDocTransition(currentStatus, targetStatus string) error {
	switch targetStatus {
	case model.FinanceDocStatusToApprove:
		if currentStatus != model.FinanceDocStatusDraft && currentStatus != model.FinanceDocStatusRejected {
			return errs.New(errs.CodeInvalidStatusTransition, errs.MsgOnlyDraftOrRejectedToToApprove)
		}
	case model.FinanceDocStatusApproved, model.FinanceDocStatusRejected:
		if currentStatus != model.FinanceDocStatusToApprove {
			return errs.New(errs.CodeInvalidStatusTransition, errs.MsgOnlyToApproveToApprovedOrRejected)
		}
	default:
		return errs.New(errs.CodeInvalidStatusTransition, errs.MsgUnsupportedTargetStatus)
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
