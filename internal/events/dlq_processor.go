package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
)

type DLQProcessor struct {
	consumer    sarama.ConsumerGroup
	producer    sarama.SyncProducer
	handler     OrderEventHandler
	logger      *logrus.Logger
	replayTopic string
}

type DLQMessage struct {
	Event    OrderCreatedEvent `json:"event"`
	Metadata MessageMetadata   `json:"metadata"`
}

func NewDLQProcessor(brokers string, handler OrderEventHandler, logger *logrus.Logger) (*DLQProcessor, error) {
	// Consumer config
	consumerConfig := sarama.NewConfig()
	consumerConfig.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	consumerConfig.Version = sarama.V2_6_0_0

	consumer, err := sarama.NewConsumerGroup([]string{brokers}, "dlq-processor-group", consumerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create DLQ consumer: %w", err)
	}

	// Producer config for replay
	producerConfig := sarama.NewConfig()
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producerConfig.Producer.Retry.Max = 5
	producerConfig.Producer.Return.Successes = true
	producerConfig.Version = sarama.V2_6_0_0

	producer, err := sarama.NewSyncProducer([]string{brokers}, producerConfig)
	if err != nil {
		consumer.Close()
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	return &DLQProcessor{
		consumer:    consumer,
		producer:    producer,
		handler:     handler,
		logger:      logger,
		replayTopic: OrderCreatedTopic,
	}, nil
}

func (p *DLQProcessor) ProcessDLQ(ctx context.Context) error {
	handler := &dlqConsumerHandler{
		processor: p,
		logger:    p.logger,
	}

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("DLQ processor context cancelled")
			return nil
		default:
			if err := p.consumer.Consume(ctx, []string{OrderCreatedDLQTopic}, handler); err != nil {
				p.logger.WithError(err).Error("Error consuming from DLQ")
				return err
			}
		}
	}
}

func (p *DLQProcessor) ReplayMessage(message *sarama.ConsumerMessage) error {
	// Extract metadata
	var metadata MessageMetadata
	for _, header := range message.Headers {
		if string(header.Key) == "metadata" {
			if err := json.Unmarshal(header.Value, &metadata); err != nil {
				p.logger.WithError(err).Error("Failed to unmarshal metadata")
			}
			break
		}
	}

	// Check if message should be replayed
	if metadata.RetryCount >= MaxRetries*2 {
		p.logger.WithFields(logrus.Fields{
			"order_key":   string(message.Key),
			"retry_count": metadata.RetryCount,
		}).Error("Message exceeded maximum replay attempts")
		return fmt.Errorf("exceeded maximum replay attempts")
	}

	// Create replay message
	replayMessage := &sarama.ProducerMessage{
		Topic: p.replayTopic,
		Key:   sarama.ByteEncoder(message.Key),
		Value: sarama.ByteEncoder(message.Value),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("retry_count"),
				Value: []byte(fmt.Sprintf("%d", metadata.RetryCount)),
			},
			{
				Key:   []byte("replayed_from_dlq"),
				Value: []byte("true"),
			},
			{
				Key:   []byte("replay_time"),
				Value: []byte(time.Now().Format(time.RFC3339)),
			},
		},
	}

	// Send to replay topic
	partition, offset, err := p.producer.SendMessage(replayMessage)
	if err != nil {
		return fmt.Errorf("failed to replay message: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"replay_topic":     p.replayTopic,
		"replay_partition": partition,
		"replay_offset":    offset,
		"order_key":        string(message.Key),
	}).Info("Message replayed from DLQ")

	return nil
}

func (p *DLQProcessor) GetDLQStats() (map[string]interface{}, error) {
	// This would typically query Kafka for DLQ statistics
	stats := map[string]interface{}{
		"dlq_topic": OrderCreatedDLQTopic,
		"status":    "monitoring",
		"timestamp": time.Now(),
	}
	
	return stats, nil
}

func (p *DLQProcessor) Close() error {
	if err := p.producer.Close(); err != nil {
		p.logger.WithError(err).Error("Failed to close producer")
	}
	return p.consumer.Close()
}

// DLQ consumer handler
type dlqConsumerHandler struct {
	processor *DLQProcessor
	logger    *logrus.Logger
}

func (h *dlqConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	h.logger.Info("DLQ consumer session setup")
	return nil
}

func (h *dlqConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.logger.Info("DLQ consumer session cleanup")
	return nil
}

func (h *dlqConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
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
			}).Info("Processing DLQ message")

			// Extract metadata to decide action
			var metadata MessageMetadata
			for _, header := range message.Headers {
				if string(header.Key) == "metadata" {
					json.Unmarshal(header.Value, &metadata)
					break
				}
			}

			// Log DLQ message details
			h.logger.WithFields(logrus.Fields{
				"original_topic": metadata.OriginalTopic,
				"retry_count":    metadata.RetryCount,
				"first_failure":  metadata.FirstFailure,
				"last_failure":   metadata.LastFailure,
				"error_message":  metadata.ErrorMessage,
			}).Warn("DLQ message details")

			// For demo purposes, we'll attempt to replay after a delay
			// In production, this might be triggered manually or by schedule
			time.Sleep(30 * time.Second)
			
			if err := h.processor.ReplayMessage(message); err != nil {
				h.logger.WithError(err).Error("Failed to replay DLQ message")
			}

			// Mark message as processed
			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
}