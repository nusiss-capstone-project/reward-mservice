package proxy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	assetclient "github.com/nusiss-capstone-project/asset-mservice/client"
	"github.com/nusiss-capstone-project/asset-mservice/common/assetpb"
	"github.com/nusiss-capstone-project/reward-mservice/server/config"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/util"
	"google.golang.org/grpc"
)

type assetRewardClient interface {
	Reward(ctx context.Context, in *assetpb.RewardRequest, opts ...grpc.CallOption) (*assetpb.RewardResponse, error)
}

type voucherIssuerImpl struct {
	assetClient assetRewardClient
}

var (
	voucherIssuerOnce sync.Once
	voucherIssuerInst VoucherIssuer
)

func GetVoucherIssuer() VoucherIssuer {
	voucherIssuerOnce.Do(func() {
		cfg := config.Config.AssetGrpcConfig
		if cfg == nil {
			log.Logger.Warnw("asset_grpc config is nil, crypto voucher issue will fail")
			voucherIssuerInst = &voucherIssuerImpl{}
			return
		}
		client, err := assetclient.GetAssetServiceClient(&assetclient.GRpcClientConfig{
			Host: cfg.Host,
			Port: cfg.Port,
		})
		if err != nil {
			panic(fmt.Sprintf("init asset grpc client: %v", err))
		}
		voucherIssuerInst = &voucherIssuerImpl{assetClient: client}
		log.Logger.Infow("asset grpc client initialized", "host", cfg.Host, "port", cfg.Port)
	})
	return voucherIssuerInst
}

func (v *voucherIssuerImpl) Issue(
	ctx context.Context,
	record *IssueRecordSnapshot,
	amount string,
) (businessSuccess bool, failedReason string, err error) {
	if record == nil {
		err := errors.New("issue record snapshot is required")
		log.WithContext(ctx).Errorw("voucher issue rejected", "error", err)
		return false, "", err
	}
	if record.VoucherType != util.VoucherTypeCrypto {
		log.WithContext(ctx).Infow("voucher issue skipped for non-crypto type",
			"voucher_type", record.VoucherType,
			"voucher_id", record.VoucherID,
		)
		return true, "", nil
	}
	return v.issueCryptoReward(ctx, record, amount)
}

func (v *voucherIssuerImpl) issueCryptoReward(
	ctx context.Context,
	record *IssueRecordSnapshot,
	amount string,
) (bool, string, error) {
	if v.assetClient == nil {
		err := errors.New("asset grpc client is not initialized")
		log.WithContext(ctx).Errorw("asset reward skipped",
			"voucher_id", record.VoucherID,
			"error", err,
		)
		return false, "", err
	}
	assetCode, err := assetCodeFromUnit(record.Unit)
	if err != nil {
		log.WithContext(ctx).Warnw("asset reward unsupported unit",
			"voucher_id", record.VoucherID,
			"unit", record.Unit,
			"error", err,
		)
		return false, err.Error(), nil
	}

	req := &assetpb.RewardRequest{
		BizId:     record.VoucherID,
		UserId:    record.UserID,
		AssetCode: assetCode,
		Amount:    amount,
	}
	log.WithContext(ctx).Infow("calling asset reward",
		"biz_id", req.BizId,
		"user_id", req.UserId,
		"asset_code", req.AssetCode,
		"amount", req.Amount,
	)

	resp, err := v.assetClient.Reward(ctx, req)
	if err != nil {
		log.WithContext(ctx).Errorw("asset reward grpc failed",
			"biz_id", req.BizId,
			"error", err,
		)
		return false, "", err
	}
	if resp == nil || resp.GetBaseInfo() == nil {
		err := errors.New("asset reward response is empty")
		log.WithContext(ctx).Errorw("asset reward invalid response",
			"biz_id", req.BizId,
			"error", err,
		)
		return false, "", err
	}

	code := resp.GetBaseInfo().GetCode()
	message := resp.GetBaseInfo().GetMessage()
	if code == assetpb.ErrorCode_ERROR_CODE_OK {
		log.WithContext(ctx).Infow("asset reward succeeded",
			"biz_id", req.BizId,
			"transaction_id", resp.GetTransactionId(),
		)
		return true, "", nil
	}

	// Permanent business failures: finalize as FAILED without retry.
	if code == assetpb.ErrorCode_ERROR_CODE_INVALID_ARGUMENT ||
		code == assetpb.ErrorCode_ERROR_CODE_NOT_FOUND {
		reason := message
		if reason == "" {
			reason = fmt.Sprintf("asset reward failed: %s", code.String())
		}
		log.WithContext(ctx).Warnw("asset reward business failure",
			"biz_id", req.BizId,
			"code", code.String(),
			"message", message,
		)
		return false, reason, nil
	}

	// Transient / unknown: keep issue_record PENDING and retry.
	err = fmt.Errorf("asset reward failed: code=%s message=%s", code.String(), message)
	log.WithContext(ctx).Errorw("asset reward transient failure",
		"biz_id", req.BizId,
		"error", err,
	)
	return false, "", err
}

func assetCodeFromUnit(unit string) (string, error) {
	unit = strings.TrimSpace(unit)
	const prefix = "CRYPTO_"
	if !strings.HasPrefix(unit, prefix) {
		return "", fmt.Errorf("unsupported crypto unit: %s", unit)
	}
	code := strings.TrimPrefix(unit, prefix)
	if code == "" {
		return "", fmt.Errorf("unsupported crypto unit: %s", unit)
	}
	return code, nil
}
