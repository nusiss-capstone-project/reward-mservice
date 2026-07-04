package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

const IssueRequestUpdatedTopic = "reward.issue_request.updated"

type IssueRequestUpdatedEvent struct {
	IssueRequestID int64 `json:"issue_request_id"`
}

type IssueRequestUpdatedProducer interface {
	PublishIssueRequestUpdated(ctx context.Context, issueRequestID int64) error
}

type issueRequestUpdatedProducerImpl struct {
	producer KafkaProducer
	topic    string
}

var (
	issueRequestUpdatedProducerOnce sync.Once
	issueRequestUpdatedProducerInst IssueRequestUpdatedProducer
)

func GetIssueRequestUpdatedProducer() IssueRequestUpdatedProducer {
	issueRequestUpdatedProducerOnce.Do(func() {
		issueRequestUpdatedProducerInst = &issueRequestUpdatedProducerImpl{
			producer: GetKafkaProducer(),
			topic:    IssueRequestUpdatedTopic,
		}
	})
	return issueRequestUpdatedProducerInst
}

func (p *issueRequestUpdatedProducerImpl) PublishIssueRequestUpdated(ctx context.Context, issueRequestID int64) error {
	if issueRequestID <= 0 {
		return errors.New("issue_request_id must be positive")
	}

	event := IssueRequestUpdatedEvent{IssueRequestID: issueRequestID}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal issue request updated event: %w", err)
	}
	key := []byte(fmt.Sprintf("%d", issueRequestID))
	return p.producer.Publish(ctx, p.topic, key, payload)
}
