package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
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
	if cfg.CreateTopic {
		if err := ensureTopic(cfg); err != nil {
			return nil, err
		}
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

func ensureTopic(cfg config.KafkaConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", cfg.Brokers[0])
	if err != nil {
		return fmt.Errorf("dial broker: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("fetch controller: %w", err)
	}

	controllerAddr := net.JoinHostPort(controller.Host, fmt.Sprintf("%d", controller.Port))
	ctrlConn, err := kafka.DialContext(ctx, "tcp", controllerAddr)
	if err != nil {
		return fmt.Errorf("dial controller: %w", err)
	}
	defer ctrlConn.Close()

	err = ctrlConn.CreateTopics(kafka.TopicConfig{
		Topic:             cfg.Topic,
		NumPartitions:     cfg.TopicPartitions,
		ReplicationFactor: cfg.TopicReplicationFactor,
	})
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "already exists") {
		return nil
	}
	return fmt.Errorf("create topic %q: %w", cfg.Topic, err)
}
