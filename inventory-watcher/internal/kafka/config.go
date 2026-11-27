package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

const (
	// ReaderModeLegacy consumes the three static PoC topics
	// (<prefix>.lifecycle, <prefix>.heartbeat, <prefix>.inference).
	ReaderModeLegacy = "legacy"

	// ReaderModeOSACReader connects directly to OSAC's Strimzi Kafka cluster
	// as an external metering.readers consumer, subscribing dynamically via
	// regex to per-tenant osac.metering.heartbeat.<tenant> topics while
	// excluding internal system/shared topics.
	ReaderModeOSACReader = "osac-reader"

	// DefaultHeartbeatPrefix is OSAC's per-tenant heartbeat topic prefix.
	DefaultHeartbeatPrefix = "osac.metering.heartbeat."
)

// Config holds Kafka connection, security (TLS/SCRAM), and topic settings.
type Config struct {
	Brokers          []string // from KAFKA_BROKERS (comma-separated)
	ConsumerGroup    string   // KAFKA_CONSUMER_GROUP
	TopicPrefix      string   // KAFKA_TOPIC_PREFIX, default "osac.metering"
	ProducerEnabled  bool     // KAFKA_MODE includes producer
	ConsumerEnabled  bool     // KAFKA_MODE includes consumer
	ReaderMode       string   // "legacy" (default) or "osac-reader"
	HeartbeatPrefix  string   // default "osac.metering.heartbeat."
	IncludeInference bool     // also consume <TopicPrefix>.inference in osac-reader mode
	TLSEnabled       bool     // enable TLS (single-cluster Strimzi or cross-cluster external listener)
	TLSCACert        string   // optional path to PEM CA bundle
	SASLUsername     string   // SCRAM-SHA-512 username (Strimzi KafkaUser)
	SASLPasswordFile string   // path to mounted password file (supports live secret rotation)
	SASLPassword     string   // inline password fallback (prefer SASLPasswordFile)
}

// NormalizedReaderMode returns ReaderModeOSACReader if configured, otherwise
// ReaderModeLegacy.
func (c Config) NormalizedReaderMode() string {
	if strings.EqualFold(strings.TrimSpace(c.ReaderMode), ReaderModeOSACReader) {
		return ReaderModeOSACReader
	}
	return ReaderModeLegacy
}

// EffectiveHeartbeatPrefix returns the configured heartbeat topic prefix or
// DefaultHeartbeatPrefix.
func (c Config) EffectiveHeartbeatPrefix() string {
	if p := strings.TrimSpace(c.HeartbeatPrefix); p != "" {
		return p
	}
	return DefaultHeartbeatPrefix
}

// TopicLifecycle returns the topic name for legacy lifecycle events.
func (c Config) TopicLifecycle() string { return c.TopicPrefix + ".lifecycle" }

// TopicHeartbeat returns the topic name for legacy heartbeat events.
func (c Config) TopicHeartbeat() string { return c.TopicPrefix + ".heartbeat" }

// TopicInference returns the topic name for inference events.
func (c Config) TopicInference() string { return c.TopicPrefix + ".inference" }

// Topics returns the static legacy topic list derived from TopicPrefix.
func (c Config) Topics() []string {
	return []string{c.TopicLifecycle(), c.TopicHeartbeat(), c.TopicInference()}
}

// OSACReaderTopicPatterns returns the regex include and exclude patterns used
// in osac-reader mode. Excluding system and shared avoids Strimzi deny ACLs
// provisioned by OSAC metering.readers.
func (c Config) OSACReaderTopicPatterns() (include, exclude []string) {
	prefix := regexp.QuoteMeta(c.EffectiveHeartbeatPrefix())
	include = []string{"^" + prefix + ".+"}
	if c.IncludeInference {
		infTopic := c.TopicInference()
		if infTopic == ".inference" {
			infTopic = "osac.metering.inference"
		}
		include = append(include, "^"+regexp.QuoteMeta(infTopic)+"$")
	}
	exclude = []string{"^" + prefix + "(?:system|shared)$"}
	return include, exclude
}

// BuildClientOptions builds shared franz-go seed broker, TLS, and SCRAM-SHA-512
// options used by both Consumer and Producer.
func (c Config) BuildClientOptions() ([]kgo.Opt, error) {
	brokers := cleanBrokers(c.Brokers)
	if len(brokers) == 0 {
		return nil, errors.New("at least one Kafka broker is required")
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
	}

	if c.TLSEnabled {
		tlsCfg, err := buildTLSConfig(c.TLSCACert)
		if err != nil {
			return nil, err
		}
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}

	if c.SASLUsername != "" {
		// Validate credentials eagerly at startup so misconfigured secrets fail fast.
		if _, err := c.resolveSASLPassword(); err != nil {
			return nil, err
		}
		user := c.SASLUsername
		opts = append(opts, kgo.SASL(scram.Sha512(func(context.Context) (scram.Auth, error) {
			pass, err := c.resolveSASLPassword()
			if err != nil {
				return scram.Auth{}, err
			}
			return scram.Auth{User: user, Pass: pass}, nil
		})))
	}

	return opts, nil
}

func (c Config) resolveSASLPassword() (string, error) {
	if c.SASLPasswordFile != "" {
		raw, err := os.ReadFile(c.SASLPasswordFile)
		if err != nil {
			return "", fmt.Errorf("read KAFKA_SASL_PASSWORD_FILE %q: %w", c.SASLPasswordFile, err)
		}
		pass := strings.TrimSpace(string(raw))
		if pass == "" {
			return "", fmt.Errorf("KAFKA_SASL_PASSWORD_FILE %q is empty", c.SASLPasswordFile)
		}
		return pass, nil
	}
	if c.SASLPassword != "" {
		return c.SASLPassword, nil
	}
	return "", errors.New("KAFKA_SASL_PASSWORD_FILE or KAFKA_SASL_PASSWORD is required when KAFKA_SASL_USERNAME is set")
}

func buildTLSConfig(caCertPath string) (*tls.Config, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if caCertPath != "" {
		pemBytes, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("read KAFKA_TLS_CA_CERT %q: %w", caCertPath, err)
		}
		if !rootCAs.AppendCertsFromPEM(pemBytes) {
			return nil, fmt.Errorf("KAFKA_TLS_CA_CERT %q contains no valid PEM certificates", caCertPath)
		}
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}, nil
}

func cleanBrokers(in []string) []string {
	out := make([]string, 0, len(in))
	for _, b := range in {
		if trimmed := strings.TrimSpace(b); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
