package events

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
)

type OrderEventHandler interface {
	HandleOrderCreated(event OrderCreatedEvent) error
}

type KafkaConsumer struct {
	consumerGroup sarama.ConsumerGroup
	handler       OrderEventHandler
	logger        *logrus.Logger
	topics        []string
}

type consumerGroupHandler struct {
	handler OrderEventHandler
	logger  *logrus.Logger
}

func NewKafkaConsumer(brokers, groupID string, handler OrderEventHandler, logger *logrus.Logger) (*KafkaConsumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Version = sarama.V2_6_0_0

	consumerGroup, err := sarama.NewConsumerGroup(strings.Split(brokers, ","), groupID, config)
	if err != nil {
		return nil, err
	}

	return &KafkaConsumer{
		consumerGroup: consumerGroup,
		handler:       handler,
		logger:        logger,
		topics:        []string{OrderCreatedTopic},
	}, nil
}

func (c *KafkaConsumer) Start(ctx context.Context) error {
	handler := &consumerGroupHandler{
		handler: c.handler,
		logger:  c.logger,
	}

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Kafka consumer context cancelled")
			return nil
		default:
			if err := c.consumerGroup.Consume(ctx, c.topics, handler); err != nil {
				c.logger.WithError(err).Error("Error consuming from Kafka")
				return err
			}
		}
	}
}

func (c *KafkaConsumer) Close() error {
	return c.consumerGroup.Close()
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	h.logger.Info("Kafka consumer group session setup")
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.logger.Info("Kafka consumer group session cleanup")
	return nil
}

// ConsumeClaim starts a consumer loop of ConsumerGroupClaim's Messages()
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			h.logger.WithFields(logrus.Fields{
				"topic":     message.Topic,
				"partition": message.Partition,
				"offset":    message.Offset,
				"key":       string(message.Key),
			}).Info("Received Kafka message")

			if err := h.handleMessage(message); err != nil {
				h.logger.WithError(err).Error("Failed to handle message")
				// Continue processing other messages even if one fails
			} else {
				// Mark message as processed
				session.MarkMessage(message, "")
			}

		case <-session.Context().Done():
			h.logger.Info("Consumer group session context cancelled")
			return nil
		}
	}
}

func (h *consumerGroupHandler) handleMessage(message *sarama.ConsumerMessage) error {
	switch message.Topic {
	case OrderCreatedTopic:
		var event OrderCreatedEvent
		if err := json.Unmarshal(message.Value, &event); err != nil {
			h.logger.WithError(err).Error("Failed to unmarshal order created event")
			return err
		}

		h.logger.WithField("order_id", event.OrderID).Info("Processing order created event")
		return h.handler.HandleOrderCreated(event)

	default:
		h.logger.WithField("topic", message.Topic).Warn("Unknown topic received")
		return nil
	}
}