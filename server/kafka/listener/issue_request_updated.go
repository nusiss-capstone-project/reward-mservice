package listener

import (
	"context"
	"encoding/json"

	"github.com/nusiss-capstone-project/reward-mservice/server/kafka"
	"github.com/nusiss-capstone-project/reward-mservice/server/kafka/producer"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

func handleIssueRequestUpdated(ctx context.Context, msg *kafka.Message) error {
	var event producer.IssueRequestUpdatedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.WithContext(ctx).Errorw("invalid issue request updated event", "error", err)
		return err
	}

	log.WithContext(ctx).Infow("issue request updated event received",
		"issue_request_id", event.IssueRequestID,
		"topic", msg.Topic,
		"offset", msg.Offset,
	)

	return service.GetIssueRequestService().ProcessKafkaEvent(ctx, event.IssueRequestID)
}
