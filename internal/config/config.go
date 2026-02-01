package config

import (
	"errors"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Kafka    KafkaConfig   `yaml:"kafka"`
	GNMI     GNMIConfig    `yaml:"gnmi"`
	Hosts    []Host        `yaml:"hosts"`
	Interval time.Duration `yaml:"interval"`
}

type KafkaConfig struct {
	Brokers        []string      `yaml:"brokers"`
	Topic          string        `yaml:"topic"`
	BatchTimeout   time.Duration `yaml:"batch_timeout"`
	RequiredAcks   string        `yaml:"required_acks"`
	MaxMessageSize int           `yaml:"max_message_size"`
}

type GNMIConfig struct {
	Username       string        `yaml:"username"`
	Password       string        `yaml:"password"`
	DialTimeout    time.Duration `yaml:"dial_timeout"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
	Insecure       bool          `yaml:"insecure"`
	Encoding       string        `yaml:"encoding"`
	Paths          []string      `yaml:"paths"`
	Type           string        `yaml:"type"`
	TLS            TLSConfig     `yaml:"tls"`
}

type TLSConfig struct {
	CAFile             string `yaml:"ca_file"`
	CertFile           string `yaml:"cert_file"`
	KeyFile            string `yaml:"key_file"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

type Host struct {
	Name     string     `yaml:"name"`
	Address  string     `yaml:"address"`
	Target   string     `yaml:"target"`
	Username string     `yaml:"username"`
	Password string     `yaml:"password"`
	Insecure *bool      `yaml:"insecure"`
	Paths    []string   `yaml:"paths"`
	Type     string     `yaml:"type"`
	TLS      *TLSConfig `yaml:"tls"`
}

type HostResolved struct {
	Name           string
	Address        string
	Target         string
	Username       string
	Password       string
	Insecure       bool
	Paths          []string
	Type           string
	Encoding       string
	DialTimeout    time.Duration
	RequestTimeout time.Duration
	TLS            TLSConfig
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}

	cfg.Kafka.applyDefaults()
	cfg.GNMI.applyDefaults()

	if v := os.Getenv("GNMI_USERNAME"); v != "" {
		cfg.GNMI.Username = v
	}
	if v := os.Getenv("GNMI_PASSWORD"); v != "" {
		cfg.GNMI.Password = v
	}

	if len(cfg.Hosts) == 0 {
		return nil, errors.New("no hosts configured")
	}

	for i := range cfg.Hosts {
		if cfg.Hosts[i].Name == "" {
			cfg.Hosts[i].Name = cfg.Hosts[i].Address
		}
	}

	return &cfg, nil
}

func (k *KafkaConfig) applyDefaults() {
	if k.BatchTimeout == 0 {
		k.BatchTimeout = 500 * time.Millisecond
	}
	if k.RequiredAcks == "" {
		k.RequiredAcks = "all"
	}
	if k.MaxMessageSize == 0 {
		k.MaxMessageSize = 5 * 1024 * 1024
	}
}

func (g *GNMIConfig) applyDefaults() {
	if g.DialTimeout == 0 {
		g.DialTimeout = 10 * time.Second
	}
	if g.RequestTimeout == 0 {
		g.RequestTimeout = 20 * time.Second
	}
	if g.Encoding == "" {
		g.Encoding = "json_ietf"
	}
	if g.Type == "" {
		g.Type = "config"
	}
}

func (h Host) Resolve(global GNMIConfig) HostResolved {
	resolved := HostResolved{
		Name:           h.Name,
		Address:        h.Address,
		Target:         h.Target,
		Username:       h.Username,
		Password:       h.Password,
		Insecure:       global.Insecure,
		Paths:          h.Paths,
		Type:           h.Type,
		Encoding:       global.Encoding,
		DialTimeout:    global.DialTimeout,
		RequestTimeout: global.RequestTimeout,
		TLS:            global.TLS,
	}

	if resolved.Username == "" {
		resolved.Username = global.Username
	}
	if resolved.Password == "" {
		resolved.Password = global.Password
	}
	if resolved.Target == "" {
		resolved.Target = h.Name
	}
	if h.Insecure != nil {
		resolved.Insecure = *h.Insecure
	}
	if h.TLS != nil {
		resolved.TLS = *h.TLS
	}
	if len(resolved.Paths) == 0 {
		resolved.Paths = global.Paths
	}
	if resolved.Type == "" {
		resolved.Type = global.Type
	}
	if len(resolved.Paths) == 0 {
		resolved.Paths = []string{"/"}
	}

	return resolved
}
