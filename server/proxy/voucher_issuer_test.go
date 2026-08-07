package proxy

import (
	"context"
	"errors"
	"testing"

	"github.com/nusiss-capstone-project/asset-mservice/common/assetpb"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

type mockAssetRewardClient struct {
	mock.Mock
}

func (m *mockAssetRewardClient) Reward(
	ctx context.Context,
	in *assetpb.RewardRequest,
	opts ...grpc.CallOption,
) (*assetpb.RewardResponse, error) {
	args := m.Called(ctx, in)
	resp, _ := args.Get(0).(*assetpb.RewardResponse)
	return resp, args.Error(1)
}

func TestVoucherIssuerNonCryptoPassthrough(t *testing.T) {
	issuer := &voucherIssuerImpl{}
	ok, reason, err := issuer.Issue(context.Background(), &IssueRecordSnapshot{
		VoucherType: "POINTS",
		Unit:        "POINT",
	}, "1")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestVoucherIssuerCryptoSuccess(t *testing.T) {
	client := new(mockAssetRewardClient)
	issuer := &voucherIssuerImpl{assetClient: client}
	client.On("Reward", mock.Anything, mock.MatchedBy(func(req *assetpb.RewardRequest) bool {
		return req.BizId == "v1" &&
			req.UserId == 10 &&
			req.AssetCode == "USDT" &&
			req.Amount == "2.00000000"
	})).Return(&assetpb.RewardResponse{
		TransactionId: "tx-1",
		BaseInfo: &assetpb.BaseResponseInfo{
			Code:    assetpb.ErrorCode_ERROR_CODE_OK,
			Message: "ok",
		},
	}, nil).Once()

	ok, reason, err := issuer.Issue(context.Background(), &IssueRecordSnapshot{
		VoucherID:   "v1",
		UserID:      10,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
	}, "2.00000000")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, reason)
	client.AssertExpectations(t)
}

func TestVoucherIssuerCryptoBusinessFailure(t *testing.T) {
	client := new(mockAssetRewardClient)
	issuer := &voucherIssuerImpl{assetClient: client}
	client.On("Reward", mock.Anything, mock.Anything).Return(&assetpb.RewardResponse{
		BaseInfo: &assetpb.BaseResponseInfo{
			Code:    assetpb.ErrorCode_ERROR_CODE_NOT_FOUND,
			Message: "asset not found",
		},
	}, nil).Once()

	ok, reason, err := issuer.Issue(context.Background(), &IssueRecordSnapshot{
		VoucherID:   "v1",
		UserID:      10,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
	}, "1")
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "asset not found", reason)
}

func TestVoucherIssuerCryptoTransientFailure(t *testing.T) {
	client := new(mockAssetRewardClient)
	issuer := &voucherIssuerImpl{assetClient: client}
	client.On("Reward", mock.Anything, mock.Anything).Return(&assetpb.RewardResponse{
		BaseInfo: &assetpb.BaseResponseInfo{
			Code:    assetpb.ErrorCode_ERROR_CODE_INTERNAL,
			Message: "timeout",
		},
	}, nil).Once()

	ok, reason, err := issuer.Issue(context.Background(), &IssueRecordSnapshot{
		VoucherID:   "v1",
		UserID:      10,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
	}, "1")
	assert.Error(t, err)
	assert.False(t, ok)
	assert.Empty(t, reason)
}

func TestVoucherIssuerCryptoGrpcError(t *testing.T) {
	client := new(mockAssetRewardClient)
	issuer := &voucherIssuerImpl{assetClient: client}
	client.On("Reward", mock.Anything, mock.Anything).Return(nil, errors.New("dial failed")).Once()

	ok, _, err := issuer.Issue(context.Background(), &IssueRecordSnapshot{
		VoucherID:   "v1",
		UserID:      10,
		VoucherType: util.VoucherTypeCrypto,
		Unit:        util.UnitCryptoUSDT,
	}, "1")
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestAssetCodeFromUnit(t *testing.T) {
	code, err := assetCodeFromUnit("CRYPTO_USDT")
	assert.NoError(t, err)
	assert.Equal(t, "USDT", code)

	_, err = assetCodeFromUnit("USDT")
	assert.Error(t, err)
}
