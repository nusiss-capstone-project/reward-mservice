package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/log"
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
		err := errors.New("client_ref_id is required")
		log.WithContext(ctx).Errorw("publish reward distribution result event rejected",
			"voucher_id", event.VoucherID,
			"error", err,
		)
		return err
	}

	payload, err := json.Marshal(event)
	if err != nil {
		err = fmt.Errorf("marshal reward distribution result event: %w", err)
		log.WithContext(ctx).Errorw("publish reward distribution result event marshal failed",
			"client_ref_id", event.ClientRefID,
			"voucher_id", event.VoucherID,
			"error", err,
		)
		return err
	}

	log.WithContext(ctx).Infow("publishing reward distribution result event",
		"client_ref_id", event.ClientRefID,
		"voucher_id", event.VoucherID,
		"status", event.Status,
		"topic", p.topic,
	)
	key := []byte(event.ClientRefID)
	if err := p.producer.Publish(ctx, p.topic, key, payload); err != nil {
		log.WithContext(ctx).Errorw("publish reward distribution result event failed",
			"client_ref_id", event.ClientRefID,
			"voucher_id", event.VoucherID,
			"topic", p.topic,
			"error", err,
		)
		return err
	}
	log.WithContext(ctx).Infow("publish reward distribution result event succeeded",
		"client_ref_id", event.ClientRefID,
		"voucher_id", event.VoucherID,
		"status", event.Status,
		"topic", p.topic,
	)
	return nil
}
