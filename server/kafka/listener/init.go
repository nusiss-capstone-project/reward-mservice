package listener

import (
	"context"

	"github.com/nusiss-capstone-project/reward-mservice/server/config"
	"github.com/nusiss-capstone-project/reward-mservice/server/kafka"
	"github.com/nusiss-capstone-project/reward-mservice/server/kafka/producer"
	"github.com/nusiss-capstone-project/reward-mservice/server/log"
)

func Init(ctx context.Context) {
	cfg := config.Config.KafkaConfig
	producer.Ensure()
	if cfg == nil || !cfg.Enabled {
		log.Logger.Info("kafka disabled")
		return
	}
	Start(ctx, cfg)
}

const TopicFinanceDocApproved = producer.FinanceDocApprovedTopic
const TopicIssueRequestUpdated = producer.IssueRequestUpdatedTopic

func init() {
	kafka.RegisterHandler(TopicFinanceDocApproved, handleFinanceDocApproved)
	kafka.RegisterHandler(TopicIssueRequestUpdated, handleIssueRequestUpdated)
}
