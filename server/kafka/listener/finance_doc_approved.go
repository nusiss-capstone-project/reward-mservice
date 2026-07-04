package listener

import (
	"context"
	"encoding/json"

	"github.com/nusiss-capstone-project/reward-mservice/server/kafka"
	"github.com/nusiss-capstone-project/reward-mservice/server/kafka/producer"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/service"
)

func handleFinanceDocApproved(ctx context.Context, msg *kafka.Message) error {
	var event producer.FinanceDocApprovedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.WithContext(ctx).Errorw("invalid finance doc approved event", "error", err)
		return err
	}

	log.WithContext(ctx).Infow("finance doc approved event received",
		"doc_id", event.DocID,
		"project_id", event.ProjectID,
		"topic", msg.Topic,
		"offset", msg.Offset,
	)

	return service.GetProjectBudgetService().InitFromApprovedDoc(ctx, event.DocID)
}
