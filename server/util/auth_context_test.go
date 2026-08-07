package util

import (
	"context"
	"testing"

	commonauth "github.com/nusiss-capstone-project/identity-mservice/common/auth"
	"github.com/stretchr/testify/assert"
)

func TestCurrentUserIDString(t *testing.T) {
	_, err := CurrentUserIDString(context.Background())
	assert.Error(t, err)

	ctx := commonauth.WithUser(context.Background(), &commonauth.User{InternalUserID: 42, Role: commonauth.RoleCampaignOps})
	id, err := CurrentUserIDString(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "42", id)
}

func TestListCreatorFilter(t *testing.T) {
	_, err := ListCreatorFilter(context.Background())
	assert.Error(t, err)

	adminCtx := commonauth.WithUser(context.Background(), &commonauth.User{InternalUserID: 1, Role: commonauth.RoleAdmin})
	creator, err := ListCreatorFilter(adminCtx)
	assert.NoError(t, err)
	assert.Empty(t, creator)

	opsCtx := commonauth.WithUser(context.Background(), &commonauth.User{InternalUserID: 9, Role: commonauth.RoleCampaignOps})
	creator, err = ListCreatorFilter(opsCtx)
	assert.NoError(t, err)
	assert.Equal(t, "9", creator)
}
