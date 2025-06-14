package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
)

const (
	OrderCreatedDLQTopic = "order.created.dlq"
	MaxRetries          = 3
	InitialRetryDelay   = 1 * time.Second
	MaxRetryDelay       = 30 * time.Second
)

type RetryableOrderEventHandler interface {
	HandleOrderCreated(event OrderCreatedEvent) error
	IsRetryable(err error) bool
}

type KafkaConsumerWithRetry struct {
	consumerGroup sarama.ConsumerGroup
	producer      sarama.SyncProducer
	handler       RetryableOrderEventHandler
	logger        *logrus.Logger
	topics        []string
	metrics       *ConsumerMetrics
}

type ConsumerMetrics struct {
	ProcessedCount   int64
	RetryCount       int64
	DLQCount         int64
	SuccessCount     int64
	FailureCount     int64
}

type MessageMetadata struct {
	RetryCount    int       `json:"retry_count"`
	FirstFailure  time.Time `json:"first_failure"`
	LastFailure   time.Time `json:"last_failure"`
	OriginalTopic string    `json:"original_topic"`
	ErrorMessage  string    `json:"error_message"`
}

type consumerGroupHandlerWithRetry struct {
	handler  RetryableOrderEventHandler
	producer sarama.SyncProducer
	logger   *logrus.Logger
	metrics  *ConsumerMetrics
}

func NewKafkaConsumerWithRetry(brokers, groupID string, handler RetryableOrderEventHandler, logger *logrus.Logger) (*KafkaConsumerWithRetry, error) {
	// Consumer config
	consumerConfig := sarama.NewConfig()
	consumerConfig.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	consumerConfig.Version = sarama.V2_6_0_0

	consumerGroup, err := sarama.NewConsumerGroup(strings.Split(brokers, ","), groupID, consumerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	// Producer config for DLQ
	producerConfig := sarama.NewConfig()
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producerConfig.Producer.Retry.Max = 5
	producerConfig.Producer.Return.Successes = true
	producerConfig.Version = sarama.V2_6_0_0

	producer, err := sarama.NewSyncProducer(strings.Split(brokers, ","), producerConfig)
	if err != nil {
		consumerGroup.Close()
		return nil, fmt.Errorf("failed to create producer for DLQ: %w", err)
	}

	return &KafkaConsumerWithRetry{
		consumerGroup: consumerGroup,
		producer:      producer,
		handler:       handler,
		logger:        logger,
		topics:        []string{OrderCreatedTopic},
		metrics:       &ConsumerMetrics{},
	}, nil
}

func (c *KafkaConsumerWithRetry) Start(ctx context.Context) error {
	handler := &consumerGroupHandlerWithRetry{
		handler:  c.handler,
		producer: c.producer,
		logger:   c.logger,
		metrics:  c.metrics,
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

func (c *KafkaConsumerWithRetry) Close() error {
	if err := c.producer.Close(); err != nil {
		c.logger.WithError(err).Error("Failed to close producer")
	}
	return c.consumerGroup.Close()
}

func (c *KafkaConsumerWithRetry) GetMetrics() ConsumerMetrics {
	return *c.metrics
}

// Consumer group handler implementation
func (h *consumerGroupHandlerWithRetry) Setup(sarama.ConsumerGroupSession) error {
	h.logger.Info("Kafka consumer group session setup")
	return nil
}

func (h *consumerGroupHandlerWithRetry) Cleanup(sarama.ConsumerGroupSession) error {
	h.logger.Info("Kafka consumer group session cleanup")
	return nil
}

func (h *consumerGroupHandlerWithRetry) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			h.metrics.ProcessedCount++
			
			if err := h.handleMessageWithRetry(message); err != nil {
				h.logger.WithError(err).Error("Failed to process message after retries")
				h.metrics.FailureCount++
				
				// Send to DLQ
				if dlqErr := h.sendToDLQ(message, err); dlqErr != nil {
					h.logger.WithError(dlqErr).Error("Failed to send message to DLQ")
				} else {
					h.metrics.DLQCount++
				}
			} else {
				h.metrics.SuccessCount++
			}

			// Mark message as processed
			session.MarkMessage(message, "")

		case <-session.Context().Done():
			h.logger.Info("Consumer group session context cancelled")
			return nil
		}
	}
}

func (h *consumerGroupHandlerWithRetry) handleMessageWithRetry(message *sarama.ConsumerMessage) error {
	h.logger.WithFields(logrus.Fields{
		"topic":     message.Topic,
		"partition": message.Partition,
		"offset":    message.Offset,
		"key":       string(message.Key),
	}).Info("Processing Kafka message with retry support")

	// Parse message metadata from headers
	metadata := h.extractMetadata(message)
	
	// Unmarshal the event
	var event OrderCreatedEvent
	if err := json.Unmarshal(message.Value, &event); err != nil {
		h.logger.WithError(err).Error("Failed to unmarshal order created event")
		return err // Non-retryable error
	}

	// Attempt to process with exponential backoff
	retryDelay := InitialRetryDelay
	
	for attempt := 0; attempt <= metadata.RetryCount + MaxRetries; attempt++ {
		if attempt > 0 {
			h.logger.WithFields(logrus.Fields{
				"order_id": event.OrderID,
				"attempt":  attempt,
				"delay":    retryDelay,
			}).Info("Retrying order processing")
			
			time.Sleep(retryDelay)
			h.metrics.RetryCount++
			
			// Exponential backoff
			retryDelay = retryDelay * 2
			if retryDelay > MaxRetryDelay {
				retryDelay = MaxRetryDelay
			}
		}

		// Attempt to process
		err := h.handler.HandleOrderCreated(event)
		if err == nil {
			h.logger.WithField("order_id", event.OrderID).Info("Successfully processed order after retries")
			return nil
		}

		// Check if error is retryable
		if !h.handler.IsRetryable(err) {
			h.logger.WithError(err).Error("Non-retryable error encountered")
			return err
		}

		h.logger.WithError(err).WithField("attempt", attempt+1).Warn("Retryable error processing order")
	}

	return fmt.Errorf("exhausted retries for order %s", event.OrderID)
}

func (h *consumerGroupHandlerWithRetry) extractMetadata(message *sarama.ConsumerMessage) MessageMetadata {
	metadata := MessageMetadata{
		RetryCount:    0,
		OriginalTopic: message.Topic,
	}

	// Extract retry count from headers if present
	for _, header := range message.Headers {
		if string(header.Key) == "retry_count" {
			// Parse retry count
			var count int
			if err := json.Unmarshal(header.Value, &count); err == nil {
				metadata.RetryCount = count
			}
		}
	}

	return metadata
}

func (h *consumerGroupHandlerWithRetry) sendToDLQ(message *sarama.ConsumerMessage, processingError error) error {
	// Create metadata for DLQ message
	metadata := MessageMetadata{
		RetryCount:    h.extractMetadata(message).RetryCount + 1,
		FirstFailure:  time.Now(),
		LastFailure:   time.Now(),
		OriginalTopic: message.Topic,
		ErrorMessage:  processingError.Error(),
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Create DLQ message with original payload and metadata
	dlqMessage := &sarama.ProducerMessage{
		Topic: OrderCreatedDLQTopic,
		Key:   sarama.ByteEncoder(message.Key),
		Value: sarama.ByteEncoder(message.Value),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("metadata"),
				Value: metadataBytes,
			},
			{
				Key:   []byte("original_topic"),
				Value: []byte(message.Topic),
			},
			{
				Key:   []byte("original_partition"),
				Value: []byte(fmt.Sprintf("%d", message.Partition)),
			},
			{
				Key:   []byte("original_offset"),
				Value: []byte(fmt.Sprintf("%d", message.Offset)),
			},
			{
				Key:   []byte("failure_time"),
				Value: []byte(time.Now().Format(time.RFC3339)),
			},
		},
	}

	// Send to DLQ
	partition, offset, err := h.producer.SendMessage(dlqMessage)
	if err != nil {
		return fmt.Errorf("failed to send to DLQ: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"dlq_topic":     OrderCreatedDLQTopic,
		"dlq_partition": partition,
		"dlq_offset":    offset,
		"original_key":  string(message.Key),
		"error":         processingError.Error(),
	}).Warn("Message sent to dead letter queue")

	return nil
}