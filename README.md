# config-pub

Collects running configuration via gNMI and publishes JSON payloads to Kafka.

## Existing libraries

In Go, the most common building blocks are:

- `github.com/openconfig/gnmi` for gNMI protos and a basic gRPC client.
- `github.com/openconfig/gnmic` if you want a full-featured CLI and higher-level helpers.
- `github.com/segmentio/kafka-go` or Confluent's Go client for Kafka publishing.

This repo uses `github.com/openconfig/gnmi` plus `github.com/segmentio/kafka-go`.

## Quick start

1. Copy the example config to a real config file:
   - `cp config/config-pub.example.yaml config/config-pub.yaml`
2. Edit `config/config-pub.yaml` with your routers and Kafka brokers.
3. Run:
   - `go run ./cmd/config-pub -config config/config-pub.yaml`

## Output format

Each Kafka message is a JSON payload with the target and a list of updates. Values from
gNMI `json` and `json_ietf` types are emitted as JSON, other values are emitted as their
native types.

## Docker

Build the container:

- `docker build -t config-pub:dev .`

## Kubernetes

A simple deployment is under `deploy/k8s/config-pub.yaml`. It mounts a ConfigMap at
`/etc/config-pub/config.yaml`. You can mount TLS materials or credentials via Secrets.

## Configuration

See `config/config-pub.example.yaml` for full config options. Defaults:

- gNMI encoding: `json_ietf`
- gNMI type: `config`
- Dial timeout: `10s`
- Request timeout: `20s`
- Kafka topic auto-create: `true` (when enabled, partitions and replication factor apply)