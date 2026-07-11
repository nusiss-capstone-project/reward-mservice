package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

const RewardDistributionResultTopic = "reward.distribution.result"

type RewardDistributionResultEvent struct {
	ClientRefID       string `json:"client_ref_id"`
	VoucherID         string `json:"voucher_id"`
	Status            string `json:"status"`
	DistributedAmount string `json:"distributed_amount"`
	FailedReason      string `json:"failed_reason"`
}

type RewardDistributionResultProducer interface {
	PublishResult(ctx context.Context, event RewardDistributionResultEvent) error
}

type rewardDistributionResultProducerImpl struct {
	producer KafkaProducer
	topic    string
}

var (
	rewardDistributionResultProducerOnce sync.Once
	rewardDistributionResultProducerInst RewardDistributionResultProducer
)

func GetRewardDistributionResultProducer() RewardDistributionResultProducer {
	rewardDistributionResultProducerOnce.Do(func() {
		rewardDistributionResultProducerInst = &rewardDistributionResultProducerImpl{
			producer: GetKafkaProducer(),
			topic:    RewardDistributionResultTopic,
		}
	})
	return rewardDistributionResultProducerInst
}

func (p *rewardDistributionResultProducerImpl) PublishResult(
	ctx context.Context,
	event RewardDistributionResultEvent,
) error {
	if event.ClientRefID == "" {
		return errors.New("client_ref_id is required")
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal reward distribution result event: %w", err)
	}
	key := []byte(event.ClientRefID)
	return p.producer.Publish(ctx, p.topic, key, payload)
}
