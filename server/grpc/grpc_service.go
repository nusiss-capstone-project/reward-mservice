package grpc

import (
	"context"

	"github.com/nusiss-capstone-project/reward-mservice/common/rewardpb"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
)

type RewardService struct {
	rewardpb.UnimplementedRewardServiceServer
}

func (s *RewardService) SayHello(ctx context.Context, in *rewardpb.HelloRequest) (*rewardpb.HelloResponse, error) {
	log.Logger.Infof("Received: %v", in.GetName())
	return &rewardpb.HelloResponse{Message: "Hello " + in.GetName()}, nil
}
