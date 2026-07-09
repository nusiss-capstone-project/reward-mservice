package service

import (
	"strings"

	"github.com/nusiss-capstone-project/reward-mservice/common/rewardpb"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
)

func validateRewardVoucherType(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == rewardpb.VoucherType_VOUCHER_TYPE_UNSPECIFIED.String() {
		return "", errs.New(errs.CodeInvalidRequest, "voucher_type is required")
	}
	if _, ok := rewardpb.VoucherType_value[name]; !ok {
		return "", errs.New(errs.CodeInvalidRequest, "invalid voucher_type")
	}
	return name, nil
}

func validateRewardUnit(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == rewardpb.Unit_UNIT_UNSPECIFIED.String() {
		return "", errs.New(errs.CodeInvalidRequest, "unit is required")
	}
	if _, ok := rewardpb.Unit_value[name]; !ok {
		return "", errs.New(errs.CodeInvalidRequest, "invalid unit")
	}
	return name, nil
}
