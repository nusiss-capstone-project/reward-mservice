package listener

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/nusiss-capstone-project/reward-mservice/server/kafka"
	"github.com/nusiss-capstone-project/reward-mservice/server/kafka/producer"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

func handleRewardDistributionExecute(ctx context.Context, msg *kafka.Message) error {
	var event producer.RewardDistributionExecuteEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.WithContext(ctx).Errorw("invalid reward distribution execute event", "error", err)
		return err
	}

	log.WithContext(ctx).Infow("reward distribution execute event received",
		"reward_request_id", event.RewardRequestID,
		"topic", msg.Topic,
		"offset", msg.Offset,
	)

	err := service.GetIssueRecordService().ExecuteRewardDistribution(ctx, event.RewardRequestID)
	if err == nil || errors.Is(err, service.ErrDistributionDeferred) {
		return nil
	}
	return err
}
