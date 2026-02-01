package gnmi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/jalapeno/config-pub/internal/config"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

type Collector struct {
	global config.GNMIConfig
}

type ConfigMessage struct {
	Timestamp time.Time      `json:"timestamp"`
	Target    string         `json:"target"`
	Address   string         `json:"address"`
	Encoding  string         `json:"encoding"`
	Type      string         `json:"type"`
	Updates   []ConfigUpdate `json:"updates"`
}

type ConfigUpdate struct {
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
	Type  string      `json:"value_type"`
}

func NewCollector(cfg config.GNMIConfig) *Collector {
	return &Collector{global: cfg}
}

func (c *Collector) Collect(ctx context.Context, host config.HostResolved) (*ConfigMessage, error) {
	conn, err := dial(ctx, host)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := gnmi.NewGNMIClient(conn)

	reqPaths, err := buildPaths(host.Paths)
	if err != nil {
		return nil, err
	}

	req := &gnmi.GetRequest{
		Prefix:   &gnmi.Path{Target: host.Target},
		Path:     reqPaths,
		Type:     parseGetType(host.Type),
		Encoding: parseEncoding(host.Encoding),
	}

	ctx, cancel := context.WithTimeout(ctx, host.RequestTimeout)
	defer cancel()

	resp, err := client.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	msg := &ConfigMessage{
		Timestamp: time.Now().UTC(),
		Target:    host.Target,
		Address:   host.Address,
		Encoding:  host.Encoding,
		Type:      host.Type,
		Updates:   []ConfigUpdate{},
	}

	for _, notification := range resp.Notification {
		for _, update := range notification.Update {
			path := gnmiPathToString(update.Path)
			value, valueType := typedValueToInterface(update.Val)
			msg.Updates = append(msg.Updates, ConfigUpdate{
				Path:  path,
				Value: value,
				Type:  valueType,
			})
		}
	}

	if len(msg.Updates) == 0 {
		msg.Updates = append(msg.Updates, ConfigUpdate{
			Path:  "/",
			Value: json.RawMessage("{}"),
			Type:  "empty",
		})
	}

	return msg, nil
}

func dial(ctx context.Context, host config.HostResolved) (*grpc.ClientConn, error) {
	creds, err := buildCredentials(host)
	if err != nil {
		return nil, err
	}

	options := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	if host.Username != "" || host.Password != "" {
		options = append(options, grpc.WithPerRPCCredentials(newBasicAuth(host.Username, host.Password, !host.Insecure)))
	}

	ctx, cancel := context.WithTimeout(ctx, host.DialTimeout)
	defer cancel()

	return grpc.DialContext(ctx, host.Address, options...)
}

func buildCredentials(host config.HostResolved) (credentials.TransportCredentials, error) {
	if host.Insecure {
		return insecure.NewCredentials(), nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: host.TLS.InsecureSkipVerify,
	}

	if host.TLS.CAFile != "" {
		caCert, err := os.ReadFile(host.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("append CA certificate: %w", err)
		}
		tlsConfig.RootCAs = caPool
	}

	if host.TLS.CertFile != "" || host.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(host.TLS.CertFile, host.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return credentials.NewTLS(tlsConfig), nil
}

func parseEncoding(enc string) gnmi.Encoding {
	switch enc {
	case "json":
		return gnmi.Encoding_JSON
	case "bytes":
		return gnmi.Encoding_BYTES
	case "proto":
		return gnmi.Encoding_PROTO
	default:
		return gnmi.Encoding_JSON_IETF
	}
}

func parseGetType(kind string) gnmi.GetRequest_DataType {
	switch kind {
	case "state":
		return gnmi.GetRequest_STATE
	case "operational":
		return gnmi.GetRequest_STATE
	case "all":
		return gnmi.GetRequest_ALL
	default:
		return gnmi.GetRequest_CONFIG
	}
}

func typedValueToInterface(value *gnmi.TypedValue) (interface{}, string) {
	if value == nil {
		return nil, "nil"
	}

	switch v := value.Value.(type) {
	case *gnmi.TypedValue_StringVal:
		return v.StringVal, "string"
	case *gnmi.TypedValue_IntVal:
		return v.IntVal, "int"
	case *gnmi.TypedValue_UintVal:
		return v.UintVal, "uint"
	case *gnmi.TypedValue_BoolVal:
		return v.BoolVal, "bool"
	case *gnmi.TypedValue_FloatVal:
		return v.FloatVal, "float"
	case *gnmi.TypedValue_DecimalVal:
		if v.DecimalVal.Precision == 0 {
			return v.DecimalVal.Digits, "decimal"
		}
		scale := math.Pow10(int(v.DecimalVal.Precision))
		return float64(v.DecimalVal.Digits) / scale, "decimal"
	case *gnmi.TypedValue_AsciiVal:
		return v.AsciiVal, "ascii"
	case *gnmi.TypedValue_BytesVal:
		return v.BytesVal, "bytes"
	case *gnmi.TypedValue_JsonVal:
		return decodeJSON(v.JsonVal), "json"
	case *gnmi.TypedValue_JsonIetfVal:
		return decodeJSON(v.JsonIetfVal), "json_ietf"
	case *gnmi.TypedValue_LeaflistVal:
		list := make([]interface{}, 0, len(v.LeaflistVal.Element))
		for _, elem := range v.LeaflistVal.Element {
			val, _ := typedValueToInterface(elem)
			list = append(list, val)
		}
		return list, "leaflist"
	case *gnmi.TypedValue_AnyVal:
		blob, err := protojson.Marshal(v.AnyVal)
		if err != nil {
			return string(blob), "any"
		}
		return json.RawMessage(blob), "any"
	default:
		blob, err := protojson.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value), "unknown"
		}
		return json.RawMessage(blob), "unknown"
	}
}

func decodeJSON(raw []byte) interface{} {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return json.RawMessage(raw)
	}
	return out
}
