package events

import (
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
)

const (
	OrderCreatedTopic = "order.created"
)

type OrderCreatedEvent struct {
	OrderID     string    `json:"order_id"`
	CustomerID  string    `json:"customer_id"`
	TotalAmount float64   `json:"total_amount"`
	CreatedAt   time.Time `json:"created_at"`
	EventTime   time.Time `json:"event_time"`
}

type KafkaProducer struct {
	producer sarama.SyncProducer
	logger   *logrus.Logger
}

func NewKafkaProducer(brokers string, logger *logrus.Logger) (*KafkaProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	config.Version = sarama.V2_6_0_0

	// Create producer
	producer, err := sarama.NewSyncProducer([]string{brokers}, config)
	if err != nil {
		return nil, err
	}

	return &KafkaProducer{
		producer: producer,
		logger:   logger,
	}, nil
}

func (p *KafkaProducer) PublishOrderCreated(event OrderCreatedEvent) error {
	// Set event time
	event.EventTime = time.Now()

	// Marshal event
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Create message
	msg := &sarama.ProducerMessage{
		Topic: OrderCreatedTopic,
		Key:   sarama.StringEncoder(event.OrderID),
		Value: sarama.ByteEncoder(data),
	}

	// Send message
	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.logger.WithError(err).Error("Failed to send message to Kafka")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"topic":     OrderCreatedTopic,
		"partition": partition,
		"offset":    offset,
		"order_id":  event.OrderID,
	}).Info("Event published to Kafka")

	return nil
}

func (p *KafkaProducer) Close() error {
	return p.producer.Close()
}