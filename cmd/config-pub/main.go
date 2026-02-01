package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jalapeno/config-pub/internal/config"
	"github.com/jalapeno/config-pub/internal/gnmi"
	"github.com/jalapeno/config-pub/internal/kafka"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "/etc/config-pub/config.yaml", "Path to config file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Print("signal received, shutting down")
		cancel()
	}()

	publisher, err := kafka.NewPublisher(cfg.Kafka)
	if err != nil {
		log.Fatalf("init kafka publisher: %v", err)
	}
	defer publisher.Close()

	collector := gnmi.NewCollector(cfg.GNMI)

	run := func() {
		log.Print("starting collection cycle")
		for _, host := range cfg.Hosts {
			hostCfg := host.Resolve(cfg.GNMI)
			msg, err := collector.Collect(ctx, hostCfg)
			if err != nil {
				log.Printf("collect failed for %s (%s): %v", hostCfg.Name, hostCfg.Address, err)
				continue
			}
			if err := publisher.Publish(ctx, hostCfg, msg); err != nil {
				log.Printf("kafka publish failed for %s (%s): %v", hostCfg.Name, hostCfg.Address, err)
				continue
			}
			log.Printf("published config for %s (%s)", hostCfg.Name, hostCfg.Address)
		}
	}

	if cfg.RunOnce {
		run()
		return
	}

	if cfg.Interval <= 0 {
		log.Print("interval not set; defaulting to 5m")
		cfg.Interval = 5 * time.Minute
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
