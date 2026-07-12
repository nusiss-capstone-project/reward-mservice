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
