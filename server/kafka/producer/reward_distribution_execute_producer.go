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

func (p *rewardDistributionExecuteProducerImpl) PublishExecute(ctx context.Context, rewardRequestID int64) error {
	if rewardRequestID <= 0 {
		err := errors.New("reward_request_id must be positive")
		log.WithContext(ctx).Errorw("publish reward distribution execute event rejected",
			"reward_request_id", rewardRequestID,
			"error", err,
		)
		return err
	}

	event := RewardDistributionExecuteEvent{RewardRequestID: rewardRequestID}
	payload, err := json.Marshal(event)
	if err != nil {
		err = fmt.Errorf("marshal reward distribution execute event: %w", err)
		log.WithContext(ctx).Errorw("publish reward distribution execute event marshal failed",
			"reward_request_id", rewardRequestID,
			"error", err,
		)
		return err
	}

	log.WithContext(ctx).Infow("publishing reward distribution execute event",
		"reward_request_id", rewardRequestID,
		"topic", p.topic,
	)
	key := []byte(fmt.Sprintf("%d", rewardRequestID))
	if err := p.producer.Publish(ctx, p.topic, key, payload); err != nil {
		log.WithContext(ctx).Errorw("publish reward distribution execute event failed",
			"reward_request_id", rewardRequestID,
			"topic", p.topic,
			"error", err,
		)
		return err
	}
	log.WithContext(ctx).Infow("publish reward distribution execute event succeeded",
		"reward_request_id", rewardRequestID,
		"topic", p.topic,
	)
	return nil
}
