package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jalapeno/config-pub/internal/config"
	"github.com/jalapeno/config-pub/internal/gnmi"
	"github.com/segmentio/kafka-go"
)

type Publisher struct {
	writer *kafka.Writer
	topic  string
	maxMsg int
}

func NewPublisher(cfg config.KafkaConfig) (*Publisher, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers are required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("kafka topic is required")
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: parseAcks(cfg.RequiredAcks),
		BatchTimeout: cfg.BatchTimeout,
	}
	return &Publisher{writer: writer, topic: cfg.Topic, maxMsg: cfg.MaxMessageSize}, nil
}

func (p *Publisher) Publish(ctx context.Context, host config.HostResolved, msg *gnmi.ConfigMessage) error {
	if msg == nil {
		return fmt.Errorf("nil message")
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if p.maxMsg > 0 && len(payload) > p.maxMsg {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(payload), p.maxMsg)
	}

	key := host.Name
	if key == "" {
		key = host.Address
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now().UTC(),
	})
}

func (p *Publisher) Close() error {
	return p.writer.Close()
}

func parseAcks(value string) kafka.RequiredAcks {
	switch strings.ToLower(value) {
	case "none", "0":
		return kafka.RequireNone
	case "one", "1":
		return kafka.RequireOne
	default:
		return kafka.RequireAll
	}
}
