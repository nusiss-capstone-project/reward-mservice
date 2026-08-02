package util

import (
	"strings"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
)

func ValidateTemplateType(raw string) (string, error) {
	t := strings.ToUpper(strings.TrimSpace(raw))
	switch t {
	case "FIX", "FIXED":
		return model.TemplateTypeFixed, nil
	case "DYNAMIC":
		return model.TemplateTypeDynamic, nil
	default:
		return "", errs.New(errs.CodeInvalidRequest, "invalid template type")
	}
}

// ValidateTemplateStatus normalizes template status. Empty input means no filter.
func ValidateTemplateStatus(raw string) (string, error) {
	status := strings.ToUpper(strings.TrimSpace(raw))
	if status == "" {
		return "", nil
	}
	switch status {
	case model.TemplateStatusDraft, model.TemplateStatusPublished:
		return status, nil
	default:
		return "", errs.New(errs.CodeInvalidRequest, "invalid template status")
	}
}
