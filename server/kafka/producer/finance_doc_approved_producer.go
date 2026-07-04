package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

const FinanceDocApprovedTopic = "reward.finance_doc.approved"

type FinanceDocApprovedEvent struct {
	DocID     string `json:"doc_id"`
	ProjectID int64  `json:"project_id"`
}

type FinanceDocApprovedProducer interface {
	PublishFinanceDocApproved(ctx context.Context, docID string, projectID int64) error
}

type financeDocApprovedProducerImpl struct {
	producer KafkaProducer
	topic    string
}

var (
	financeDocApprovedProducerOnce sync.Once
	financeDocApprovedProducerInst FinanceDocApprovedProducer
)

func GetFinanceDocApprovedProducer() FinanceDocApprovedProducer {
	financeDocApprovedProducerOnce.Do(func() {
		financeDocApprovedProducerInst = &financeDocApprovedProducerImpl{
			producer: GetKafkaProducer(),
			topic:    FinanceDocApprovedTopic,
		}
	})
	return financeDocApprovedProducerInst
}

func (p *financeDocApprovedProducerImpl) PublishFinanceDocApproved(
	ctx context.Context,
	docID string,
	projectID int64,
) error {
	if docID == "" {
		return errors.New("doc_id is required")
	}
	if projectID <= 0 {
		return errors.New("project_id must be positive")
	}

	event := FinanceDocApprovedEvent{
		DocID:     docID,
		ProjectID: projectID,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal finance doc approved event: %w", err)
	}
	return p.producer.Publish(ctx, p.topic, []byte(docID), payload)
}
