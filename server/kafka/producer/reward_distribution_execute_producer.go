package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/log"
)

const RewardDistributionExecuteTopic = "reward.distribution.execute"

type RewardDistributionExecuteEvent struct {
	RewardRequestID int64 `json:"reward_request_id"`
}

type RewardDistributionExecuteProducer interface {
	PublishExecute(ctx context.Context, rewardRequestID int64) error
}

type rewardDistributionExecuteProducerImpl struct {
	producer KafkaProducer
	topic    string
}

var (
	rewardDistributionExecuteProducerOnce sync.Once
	rewardDistributionExecuteProducerInst RewardDistributionExecuteProducer
)

func GetRewardDistributionExecuteProducer() RewardDistributionExecuteProducer {
	rewardDistributionExecuteProducerOnce.Do(func() {
		rewardDistributionExecuteProducerInst = &rewardDistributionExecuteProducerImpl{
			producer: GetKafkaProducer(),
			topic:    RewardDistributionExecuteTopic,
		}
	})
	return rewardDistributionExecuteProducerInst
}

func (p *rewardDistributionExecuteProducerImpl) PublishExecute(ctx context.Context, rewardRequestID int64) (err error) {
	if rewardRequestID <= 0 {
		return errors.New("reward_request_id must be positive")
	}

	event := RewardDistributionExecuteEvent{RewardRequestID: rewardRequestID}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal reward distribution execute event: %w", err)
	}
	key := []byte(fmt.Sprintf("%d", rewardRequestID))
	defer func() {
		log.WithContext(ctx).Infof("publish reward distribution execute event done", "reward_request_id", rewardRequestID, "error", err)
	}()
	return p.producer.Publish(ctx, p.topic, key, payload)
}
