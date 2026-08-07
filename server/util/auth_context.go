package util

import (
	"context"
	"strconv"

	commonauth "github.com/nusiss-capstone-project/identity-mservice/common/auth"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
)

// CurrentUserIDString returns the authenticated internal user id as string for creator fields.
func CurrentUserIDString(ctx context.Context) (string, error) {
	userID, ok := commonauth.GetUserID(ctx)
	if !ok || userID <= 0 {
		return "", errs.New(errs.CodeInvalidRequest, "authentication required")
	}
	return strconv.FormatInt(userID, 10), nil
}

// ListCreatorFilter returns creator filter for list APIs.
// campaign_ops can only see own records; admin / finance_admin see all.
func ListCreatorFilter(ctx context.Context) (creator string, err error) {
	user, ok := commonauth.GetUser(ctx)
	if !ok || user == nil {
		return "", errs.New(errs.CodeInvalidRequest, "authentication required")
	}
	switch user.Role {
	case commonauth.RoleAdmin, commonauth.RoleFinanceAdmin:
		return "", nil
	default:
		return strconv.FormatInt(user.InternalUserID, 10), nil
	}
}
