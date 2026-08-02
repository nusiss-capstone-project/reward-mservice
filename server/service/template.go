package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/http/data"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/dao"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"
)

type TemplateService interface {
	CreateTemplate(ctx context.Context, req *data.CreateTemplateRequest) (int64, error)
	UpdateTemplate(ctx context.Context, templateID int64, req *data.UpdateTemplateRequest) (*data.TemplateVO, error)
	ListTemplates(ctx context.Context, query data.TemplateListQuery) (*data.PageResult, error)
	PublishTemplate(ctx context.Context, templateID int64) (*data.PublishTemplateResponse, error)
}

type TemplateServiceImpl struct {
	templateDao dao.TemplateDao
}

var (
	templateServiceOnce sync.Once
	templateServiceInst TemplateService
)

func GetTemplateService() TemplateService {
	templateServiceOnce.Do(func() {
		templateServiceInst = &TemplateServiceImpl{
			templateDao: dao.GetTemplateDao(),
		}
	})
	return templateServiceInst
}

func (s *TemplateServiceImpl) CreateTemplate(ctx context.Context, req *data.CreateTemplateRequest) (int64, error) {
	logger := log.WithContext(ctx)
	if req == nil {
		return 0, templateErr(ctx, errs.New(errs.CodeInvalidRequest, errs.MsgRequestRequired),
			errs.LogInputError, "reason", "nil request")
	}

	input, err := parseTemplateInput(req.VoucherType, req.Unit, req.Type, req.Config)
	if err != nil {
		return 0, templateErr(ctx, err, errs.LogInputError)
	}

	template := &model.Template{
		VoucherType: input.voucherType,
		Unit:        input.unit,
		Type:        input.templateType,
		Config:      input.config,
		Status:      model.TemplateStatusDraft,
	}
	id, err := s.templateDao.Create(ctx, template)
	if err != nil {
		return 0, templateErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			errs.LogOperationFailed, "voucher_type", input.voucherType, "unit", input.unit)
	}
	logger.Infof("template created: id=%d type=%s", id, input.templateType)
	return id, nil
}

func (s *TemplateServiceImpl) UpdateTemplate(
	ctx context.Context,
	templateID int64,
	req *data.UpdateTemplateRequest,
) (*data.TemplateVO, error) {
	if req == nil {
		return nil, templateErr(ctx, errs.New(errs.CodeInvalidRequest, errs.MsgRequestRequired),
			errs.LogInputError, "template_id", templateID)
	}

	template, err := s.loadEditableTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}

	if len(req.Config) == 0 {
		return nil, templateErr(ctx, errs.New(errs.CodeInvalidRequest, "config is required"),
			errs.LogInputError, "template_id", templateID)
	}
	config, err := parseTemplateConfig(template.Type, req.Config)
	if err != nil {
		return nil, templateErr(ctx, err, errs.LogInputError, "template_id", templateID)
	}

	if err := s.templateDao.Update(ctx, templateID, config); err != nil {
		return nil, templateErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			errs.LogOperationFailed, "template_id", templateID)
	}

	template.Config = config
	log.WithContext(ctx).Infof("template config updated: id=%d type=%s", templateID, template.Type)
	return toTemplateVO(template)
}

func (s *TemplateServiceImpl) ListTemplates(ctx context.Context, query data.TemplateListQuery) (*data.PageResult, error) {
	logger := log.WithContext(ctx)
	page, size := query.Normalize()

	normalizedStatus, err := util.ValidateTemplateStatus(query.Status)
	if err != nil {
		return nil, templateErr(ctx, err, errs.LogInputError, "status", query.Status)
	}

	templates, total, err := s.templateDao.List(ctx, page, size, normalizedStatus)
	if err != nil {
		return nil, templateErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			errs.LogOperationFailed, "page", page, "size", size, "status", normalizedStatus)
	}

	items := make([]*data.TemplateVO, 0, len(templates))
	for _, template := range templates {
		vo, err := toTemplateVO(template)
		if err != nil {
			return nil, templateErr(ctx, errs.Wrap(errs.CodeInternalError, err),
				errs.LogOperationFailed, "template_id", template.ID)
		}
		items = append(items, vo)
	}
	logger.Infof("templates listed: page=%d size=%d total=%d status=%s", page, size, total, normalizedStatus)
	return &data.PageResult{
		Total: total,
		Page:  page,
		Size:  size,
		Items: items,
	}, nil
}

func (s *TemplateServiceImpl) PublishTemplate(
	ctx context.Context,
	templateID int64,
) (*data.PublishTemplateResponse, error) {
	template, err := s.templateDao.GetByID(ctx, templateID)
	if err != nil {
		return nil, templateErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			errs.LogOperationFailed, "template_id", templateID)
	}
	if template == nil {
		return nil, templateErr(ctx, errs.New(errs.CodeTemplateNotFound, ""),
			errs.LogInputError, "template_id", templateID)
	}
	if err := validateTemplatePublishTransition(template.Status); err != nil {
		return nil, templateErr(ctx, err, errs.LogInputError,
			"template_id", templateID, "status", template.Status)
	}

	if err := s.templateDao.UpdateStatus(
		ctx, templateID,
		model.TemplateStatusDraft, model.TemplateStatusPublished,
	); err != nil {
		if isTemplateStatusTransitionFailed(err) {
			return nil, templateErr(ctx, err, errs.LogInputError,
				"template_id", templateID, "status", template.Status)
		}
		return nil, templateErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			errs.LogOperationFailed, "template_id", templateID)
	}

	log.WithContext(ctx).Infof("template published: id=%d", templateID)
	return &data.PublishTemplateResponse{
		TemplateID: templateID,
		Status:     model.TemplateStatusPublished,
	}, nil
}

func (s *TemplateServiceImpl) loadEditableTemplate(ctx context.Context, templateID int64) (*model.Template, error) {
	if templateID <= 0 {
		return nil, templateErr(ctx, errs.New(errs.CodeInvalidRequest, "invalid template_id"),
			errs.LogInputError, "template_id", templateID)
	}

	template, err := s.templateDao.GetByID(ctx, templateID)
	if err != nil {
		return nil, templateErr(ctx, errs.Wrap(errs.CodeInternalError, err),
			errs.LogOperationFailed, "template_id", templateID)
	}
	if template == nil {
		return nil, templateErr(ctx, errs.New(errs.CodeTemplateNotFound, ""),
			errs.LogInputError, "template_id", templateID)
	}
	if err := validateTemplateEditable(template.Status); err != nil {
		return nil, templateErr(ctx, err, errs.LogInputError,
			"template_id", templateID, "status", template.Status)
	}
	return template, nil
}

type templateInput struct {
	voucherType  string
	unit         string
	templateType string
	config       []byte
}

func parseTemplateInput(voucherType, unit, templateType string, config json.RawMessage) (*templateInput, error) {
	validatedVoucherType, err := util.ValidateVoucherType(voucherType)
	if err != nil {
		return nil, err
	}
	validatedUnit, err := util.ValidateUnit(unit)
	if err != nil {
		return nil, err
	}
	normalizedType, err := util.ValidateTemplateType(templateType)
	if err != nil {
		return nil, err
	}
	if len(config) == 0 {
		return nil, errs.New(errs.CodeInvalidRequest, "config is required")
	}

	normalizedConfig, err := parseTemplateConfig(normalizedType, config)
	if err != nil {
		return nil, err
	}
	return &templateInput{
		voucherType:  validatedVoucherType,
		unit:         validatedUnit,
		templateType: normalizedType,
		config:       normalizedConfig,
	}, nil
}

func parseTemplateConfig(templateType string, raw json.RawMessage) ([]byte, error) {
	switch templateType {
	case model.TemplateTypeFixed:
		return parseFixTemplateConfig(raw)
	case model.TemplateTypeDynamic:
		return parseDynamicTemplateConfig(raw)
	default:
		return nil, errs.New(errs.CodeInvalidRequest, "invalid template type")
	}
}

func parseFixTemplateConfig(raw json.RawMessage) ([]byte, error) {
	if err := validateConfigKeys(raw, "amount"); err != nil {
		return nil, err
	}

	var payload struct {
		Amount json.RawMessage `json:"amount"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload.Amount) == 0 {
		return nil, errs.New(errs.CodeInvalidRequest, "amount is required")
	}

	amountStr, err := normalizeConfigAmount(payload.Amount)
	if err != nil {
		return nil, err
	}
	parsed, err := util.ParseAmount(amountStr)
	if err != nil || parsed.Sign() <= 0 {
		return nil, errs.New(errs.CodeInvalidRequest, errs.MsgInvalidAmount)
	}

	return json.Marshal(model.FixTemplateConfig{
		Amount: util.FormatAmount(parsed),
	})
}

func parseDynamicTemplateConfig(raw json.RawMessage) ([]byte, error) {
	if err := validateConfigKeys(raw, "base_metric", "rate", "cap"); err != nil {
		return nil, err
	}

	var cfg data.DynamicTemplateConfigVO
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, errs.New(errs.CodeInvalidRequest, "invalid config")
	}
	if strings.TrimSpace(cfg.BaseMetric) == "" {
		return nil, errs.New(errs.CodeInvalidRequest, "base_metric is required")
	}
	if cfg.Rate <= 0 {
		return nil, errs.New(errs.CodeInvalidRequest, "rate must be positive")
	}

	normalized := model.DynamicTemplateConfig{
		BaseMetric: strings.TrimSpace(cfg.BaseMetric),
		Rate:       cfg.Rate,
	}
	if strings.TrimSpace(cfg.Cap) != "" {
		capAmount, err := util.ParseAmount(cfg.Cap)
		if err != nil || capAmount.Sign() < 0 {
			return nil, errs.New(errs.CodeInvalidRequest, errs.MsgInvalidAmount)
		}
		normalized.Cap = util.FormatAmount(capAmount)
	}

	return json.Marshal(normalized)
}

func validateConfigKeys(raw json.RawMessage, allowed ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return errs.New(errs.CodeInvalidRequest, "invalid config")
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range fields {
		if _, ok := allowedSet[key]; !ok {
			return errs.New(errs.CodeInvalidRequest, "unexpected config field: "+key)
		}
	}
	return nil
}

func normalizeConfigAmount(raw json.RawMessage) (string, error) {
	var num json.Number
	if err := json.Unmarshal(raw, &num); err == nil {
		return num.String(), nil
	}
	var amount string
	if err := json.Unmarshal(raw, &amount); err != nil {
		return "", errs.New(errs.CodeInvalidRequest, errs.MsgInvalidAmount)
	}
	return strings.TrimSpace(amount), nil
}

func validateTemplateEditable(status string) error {
	if status != model.TemplateStatusDraft {
		return errs.New(errs.CodeInvalidStatusTransition, "template is not editable")
	}
	return nil
}

func validateTemplatePublishTransition(status string) error {
	if status != model.TemplateStatusDraft {
		return errs.New(errs.CodeInvalidStatusTransition, "only DRAFT can move to PUBLISHED")
	}
	return nil
}

func toTemplateVO(template *model.Template) (*data.TemplateVO, error) {
	config, err := templateConfigToVO(template.Type, template.Config)
	if err != nil {
		return nil, err
	}
	return &data.TemplateVO{
		ID:          template.ID,
		VoucherType: template.VoucherType,
		Unit:        template.Unit,
		Type:        template.Type,
		Config:      config,
		Status:      template.Status,
		CreatedAt:   util.FormatDateTime(template.CreatedAt),
		UpdatedAt:   util.FormatDateTime(template.UpdatedAt),
	}, nil
}

func templateConfigToVO(templateType string, raw []byte) (interface{}, error) {
	switch templateType {
	case model.TemplateTypeFixed:
		var cfg model.FixTemplateConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		return data.FixTemplateConfigVO{Amount: cfg.Amount}, nil
	case model.TemplateTypeDynamic:
		var cfg model.DynamicTemplateConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		return data.DynamicTemplateConfigVO{
			BaseMetric: cfg.BaseMetric,
			Rate:       cfg.Rate,
			Cap:        cfg.Cap,
		}, nil
	default:
		return nil, errs.New(errs.CodeInvalidRequest, "invalid template type")
	}
}

func ParseTemplateID(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errs.New(errs.CodeInvalidRequest, "template_id is required")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errs.New(errs.CodeInvalidRequest, "invalid template_id")
	}
	return id, nil
}

func isTemplateStatusTransitionFailed(err error) bool {
	var appErr *errs.AppError
	return errors.As(err, &appErr) && appErr.Code == errs.CodeInvalidStatusTransition
}

func templateErr(ctx context.Context, err error, logMsg string, kv ...any) error {
	log.LogAppError(ctx, err, logMsg, append([]any{"error", err}, kv...)...)
	return err
}
