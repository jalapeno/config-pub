package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jalapeno/config-pub/internal/gnmi"
	"github.com/jalapeno/config-pub/internal/ingest"
	"github.com/segmentio/kafka-go"
)

const (
	defaultIGPCollection = "igp_node"
	defaultBGPCollection = "bgp_node"
)

func main() {
	var (
		kafkaBrokers string
		kafkaTopic   string
		kafkaGroup   string

		dbURL      string
		dbName     string
		dbUser     string
		dbPass     string
		dbUserFile string
		dbPassFile string

		igpCollection string
		bgpCollection string
	)

	flag.StringVar(&kafkaBrokers, "message-server", "", "Kafka broker list (comma-separated)")
	flag.StringVar(&kafkaTopic, "kafka-topic", "gnmi-config", "Kafka topic to consume")
	flag.StringVar(&kafkaGroup, "kafka-group", "config-ingest", "Kafka consumer group id")
	flag.StringVar(&dbURL, "database-server", "", "ArangoDB endpoint, e.g. http://arangodb.jalapeno:8529")
	flag.StringVar(&dbName, "database-name", "", "ArangoDB database name")
	flag.StringVar(&dbUser, "database-user", "", "ArangoDB username")
	flag.StringVar(&dbPass, "database-pass", "", "ArangoDB password")
	flag.StringVar(&dbUserFile, "database-user-file", "", "Path to ArangoDB username file")
	flag.StringVar(&dbPassFile, "database-pass-file", "", "Path to ArangoDB password file")
	flag.StringVar(&igpCollection, "igp-collection", defaultIGPCollection, "Arango IGP node collection")
	flag.StringVar(&bgpCollection, "bgp-collection", defaultBGPCollection, "Arango BGP node collection")
	flag.Parse()

	if kafkaBrokers == "" || kafkaTopic == "" {
		log.Fatal("message-server and kafka-topic are required")
	}
	if dbURL == "" || dbName == "" {
		log.Fatal("database-server and database-name are required")
	}

	client, err := ingest.NewArangoClient(ingest.ArangoConfig{
		URL:           dbURL,
		Database:      dbName,
		User:          dbUser,
		Password:      dbPass,
		UserFile:      dbUserFile,
		PassFile:      dbPassFile,
		IGPCollection: igpCollection,
		BGPCollection: bgpCollection,
	})
	if err != nil {
		log.Fatalf("arango client: %v", err)
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        splitComma(kafkaBrokers),
		Topic:          kafkaTopic,
		GroupID:        kafkaGroup,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: time.Second,
	})
	defer reader.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Print("signal received, shutting down")
		cancel()
	}()

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("kafka read error: %v", err)
			continue
		}

		var payload gnmi.ConfigMessage
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			log.Printf("invalid payload: %v", err)
			continue
		}

		info, err := ingest.ExtractMatchInfo(payload.Updates)
		if err != nil {
			log.Printf("parse payload: %v", err)
			continue
		}

		update := map[string]interface{}{
			"running_config": payload,
			"config_ts":      payload.Timestamp,
			"config_source": map[string]interface{}{
				"target":  payload.Target,
				"address": payload.Address,
			},
		}

		switch {
		case info.HasISIS:
			ok, err := client.UpdateIGP(ctx, info.RouterID, info.Hostname, update)
			if err != nil {
				log.Printf("igp update failed for %s: %v", info.RouterID, err)
				continue
			}
			if !ok {
				log.Printf("igp node not found for router_id=%s hostname=%s", info.RouterID, info.Hostname)
				continue
			}
		case info.HasBGP:
			if info.BGPASN == 0 {
				log.Printf("bgp payload missing ASN for router_id=%s hostname=%s", info.RouterID, info.Hostname)
				continue
			}
			ok, err := client.UpdateBGP(ctx, info.RouterID, info.BGPASN, update)
			if err != nil {
				log.Printf("bgp update failed for %s: %v", info.RouterID, err)
				continue
			}
			if !ok {
				log.Printf("bgp node not found for router_id=%s asn=%d", info.RouterID, info.BGPASN)
				continue
			}
		default:
			log.Printf("payload missing IGP/BGP markers for target=%s", payload.Target)
			continue
		}

		log.Printf("stored config for target=%s router_id=%s", payload.Target, info.RouterID)
	}
}

func splitComma(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
