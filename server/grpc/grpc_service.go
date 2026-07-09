package grpc

import (
	"context"
	"errors"

	"github.com/nusiss-capstone-project/reward-mservice/common/rewardpb"
	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

type RewardService struct {
	rewardpb.UnimplementedRewardServiceServer
}

func (s *RewardService) SayHello(ctx context.Context, in *rewardpb.HelloRequest) (*rewardpb.HelloResponse, error) {
	log.Logger.Infof("Received: %v", in.GetName())
	return &rewardpb.HelloResponse{Message: "Hello " + in.GetName()}, nil
}

func (s *RewardService) Reward(
	ctx context.Context,
	in *rewardpb.RewardDistributionRequest,
) (*rewardpb.RewardDistributionResponse, error) {
	err := service.GetIssueRecordService().ProcessVoucherIssueRequest(ctx, in)
	if err != nil {
		code, message := mapRewardError(err)
		return &rewardpb.RewardDistributionResponse{
			ClientRefId: in.GetClientRefId(),
			BaseInfo: &rewardpb.BaseResponseInfo{
				Code:    code,
				Message: message,
			},
		}, nil
	}

	return &rewardpb.RewardDistributionResponse{
		ClientRefId: in.GetClientRefId(),
		BaseInfo: &rewardpb.BaseResponseInfo{
			Code:    rewardpb.ErrorCode_OK,
			Message: rewardpb.ErrorCode_OK.String(),
		},
	}, nil
}

func mapRewardError(err error) (rewardpb.ErrorCode, string) {
	var appErr *errs.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case errs.CodeDuplicateClientRefID:
			return rewardpb.ErrorCode_INVALID_PARAM, appErr.Error()
		case errs.CodeInvalidRequest:
			return rewardpb.ErrorCode_INVALID_PARAM, appErr.Error()
		case errs.CodeProjectNotFound, errs.CodeProjectBudgetNotFound:
			return rewardpb.ErrorCode_DATA_NOT_EXIST, appErr.Error()
		default:
			return rewardpb.ErrorCode_UNKNOWN_ERROR, appErr.Error()
		}
	}
	return rewardpb.ErrorCode_UNKNOWN_ERROR, err.Error()
}
