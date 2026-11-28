package config

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type Config struct {
	OSACBaseURL        string
	OSACGRPCAddress    string
	OSACToken          string
	OSACCACert         string
	InventoryDBURL     string
	ReconcileInterval  time.Duration
	MeteringInterval   time.Duration
	RatingInterval     time.Duration
	RatingBatchSize    int
	LogLevel           string
	LogFormat               string
	IngestListenAddr        string
	MetricsPort             string
	CustomMetricsConfigPath string
	AuthIssuerURL           string
	AuthTrustIdentityHeader bool
	ExportSharedSecret      string
	ExportIncludeUserID     bool
	DebugDashboard          bool
	DisabledComponents map[string]bool
	ReconcileEntities  map[string]bool
	SplunkHECURL       string
	SplunkHECToken     string
	SplunkIndex        string
	SplunkInterval     time.Duration
	SplunkTLSInsecure  bool
	KafkaBrokers          string
	KafkaConsumerGroup    string
	KafkaTopicPrefix      string
	KafkaMode             string // "both" (default in legacy), "producer", "consumer" (default in osac-reader)
	KafkaReaderMode       string // "legacy" (default) or "osac-reader" (OSAC metering.readers direct Kafka reader)
	KafkaHeartbeatPrefix  string // default "osac.metering.heartbeat."
	KafkaIncludeInference bool   // subscribe to <prefix>.inference in osac-reader mode
	KafkaTLSEnabled       bool   // TLS for Kafka; auto-enabled when CA cert or SASL username is set
	KafkaTLSCACert        string // path to PEM CA bundle for Kafka broker verification
	KafkaSASLUsername     string // SCRAM-SHA-512 username
	KafkaSASLPasswordFile string // path to file containing SCRAM-SHA-512 password
	KafkaSASLPassword     string // inline SCRAM-SHA-512 password (prefer KafkaSASLPasswordFile)
}

// DiagnosticInfo returns config values safe to expose via the debug API (no secrets).
type DiagnosticInfo struct {
	OSACBaseURL             string   `json:"osac_base_url"`
	InventoryDBHost         string   `json:"inventory_db_host"`
	ReconcileInterval       string   `json:"reconcile_interval"`
	ReconcileEntities       []string `json:"reconcile_entities,omitempty"`
	MeteringInterval        string   `json:"metering_interval"`
	RatingInterval          string   `json:"rating_interval"`
	LogLevel                string   `json:"log_level"`
	LogFormat               string   `json:"log_format"`
	IngestListenAddr        string   `json:"ingest_listen_addr"`
	MetricsPort             string   `json:"metrics_port"`
	CustomMetricsConfigPath string   `json:"custom_metrics_config_path"`
	AuthIssuerURL           string   `json:"auth_issuer_url"`
	AuthTrustIdentityHeader bool     `json:"auth_trust_identity_header"`
	ExportSharedSecretSet   bool     `json:"export_shared_secret_set"`
	ExportIncludeUserID     bool     `json:"export_include_user_id"`
	DebugDashboard          bool     `json:"debug_dashboard"`
	OSACTokenSet            bool     `json:"osac_token_set"`
	OSACCACertSet           bool     `json:"osac_ca_cert_set"`
	SplunkHECURL            string   `json:"splunk_hec_url,omitempty"`
	SplunkTokenSet          bool     `json:"splunk_token_set"`
	SplunkIndex             string   `json:"splunk_index,omitempty"`
	KafkaBrokers            string   `json:"kafka_brokers,omitempty"`
	KafkaReaderMode         string   `json:"kafka_reader_mode,omitempty"`
	KafkaTLSEnabled         bool     `json:"kafka_tls_enabled,omitempty"`
	KafkaTLSCACertSet       bool     `json:"kafka_tls_ca_cert_set,omitempty"`
	KafkaSASLConfigured     bool     `json:"kafka_sasl_configured,omitempty"`
}

// Diagnostics returns a sanitized DiagnosticInfo snapshot of the configuration.
func (c *Config) Diagnostics() DiagnosticInfo {
	var reconcileEntities []string
	for e := range c.ReconcileEntities {
		reconcileEntities = append(reconcileEntities, e)
	}
	sort.Strings(reconcileEntities)

	return DiagnosticInfo{
		OSACBaseURL:             c.OSACBaseURL,
		InventoryDBHost:         maskDBURL(c.InventoryDBURL),
		ReconcileInterval:       c.ReconcileInterval.String(),
		ReconcileEntities:       reconcileEntities,
		MeteringInterval:        c.MeteringInterval.String(),
		RatingInterval:          c.RatingInterval.String(),
		LogLevel:                c.LogLevel,
		LogFormat:               c.LogFormat,
		IngestListenAddr:        c.IngestListenAddr,
		MetricsPort:             c.MetricsPort,
		CustomMetricsConfigPath: c.CustomMetricsConfigPath,
		AuthIssuerURL:           c.AuthIssuerURL,
		AuthTrustIdentityHeader: c.AuthTrustIdentityHeader,
		ExportSharedSecretSet:   c.ExportSharedSecret != "",
		ExportIncludeUserID:     c.ExportIncludeUserID,
		DebugDashboard:          c.DebugDashboard,
		OSACTokenSet:            c.OSACToken != "",
		OSACCACertSet:           c.OSACCACert != "",
		SplunkHECURL:            c.SplunkHECURL,
		SplunkTokenSet:          c.SplunkHECToken != "",
		SplunkIndex:             c.SplunkIndex,
		KafkaBrokers:            c.KafkaBrokers,
		KafkaReaderMode:         c.KafkaReaderMode,
		KafkaTLSEnabled:         c.KafkaTLSEnabled,
		KafkaTLSCACertSet:       c.KafkaTLSCACert != "",
		KafkaSASLConfigured:     c.KafkaSASLUsername != "",
	}
}

func maskDBURL(url string) string {
	at := -1
	for i, c := range url {
		if c == '@' {
			at = i
			break
		}
	}
	if at < 0 {
		return url
	}
	scheme := ""
	slashes := 0
	for i, c := range url {
		if c == '/' {
			slashes++
			if slashes == 2 {
				scheme = url[:i+1]
				break
			}
		}
	}
	return scheme + "****@" + url[at+1:]
}


// Load reads configuration settings from environment variables with defaults.
func Load() *Config {
	readerMode := strings.ToLower(strings.TrimSpace(envOrDefault("KAFKA_READER_MODE", "legacy")))
	defaultKafkaMode := "both"
	if readerMode == "osac-reader" {
		defaultKafkaMode = "consumer"
	}
	kafkaTLSCACert := os.Getenv("KAFKA_TLS_CA_CERT")
	kafkaSASLUser := os.Getenv("KAFKA_SASL_USERNAME")
	kafkaTLSEnabled := parseKafkaTLSEnabled(os.Getenv("KAFKA_TLS_ENABLED"), kafkaTLSCACert != "" || kafkaSASLUser != "")

	return &Config{
		OSACBaseURL:             envOrDefault("OSAC_BASE_URL", "http://localhost:8011"),
		OSACGRPCAddress:         envOrDefault("OSAC_GRPC_ADDRESS", "localhost:8010"),
		OSACToken:               os.Getenv("OSAC_TOKEN"),
		OSACCACert:              envOrDefault("OSAC_CA_CERT", ""),
		InventoryDBURL:          envOrDefault("INVENTORY_DB_URL", "postgres://user:pass@localhost:5434/costdb"),
		ReconcileInterval:       durationOrDefault("RECONCILE_INTERVAL", 1*time.Hour),
		MeteringInterval:        durationOrDefault("METERING_INTERVAL", 45*time.Second),
		RatingInterval:          durationOrDefault("RATING_INTERVAL", 20*time.Second),
		RatingBatchSize:         intOrDefault("RATING_BATCH_SIZE", 2000),
		LogLevel:                envOrDefault("LOG_LEVEL", "info"),
		LogFormat:               envOrDefault("LOG_FORMAT", "text"),
		IngestListenAddr:        os.Getenv("INGEST_LISTEN_ADDR"),
		MetricsPort:             envOrDefault("METRICS_PORT", "9000"),
		CustomMetricsConfigPath: os.Getenv("CUSTOM_METRICS_CONFIG"),
		AuthIssuerURL:           os.Getenv("AUTH_ISSUER_URL"),
		AuthTrustIdentityHeader: os.Getenv("AUTH_TRUST_IDENTITY_HEADER") == "true",
		ExportSharedSecret:      os.Getenv("EXPORT_SHARED_SECRET"),
		ExportIncludeUserID:     envOrDefault("EXPORT_INCLUDE_USER_ID", "true") != "false",
		DebugDashboard:          envOrDefault("DEBUG_DASHBOARD", "true") != "false",
		DisabledComponents:      parseDisabledComponents(os.Getenv("DISABLE_COMPONENTS")),
		ReconcileEntities:       parseReconcileEntities(os.Getenv("RECONCILE_ENTITIES")),
		SplunkHECURL:            os.Getenv("SPLUNK_HEC_URL"),
		SplunkHECToken:          os.Getenv("SPLUNK_HEC_TOKEN"),
		SplunkIndex:             os.Getenv("SPLUNK_INDEX"),
		SplunkInterval:          durationOrDefault("SPLUNK_INTERVAL", 10*time.Second),
		SplunkTLSInsecure:       os.Getenv("SPLUNK_TLS_INSECURE") == "true",
		KafkaBrokers:            os.Getenv("KAFKA_BROKERS"),
		KafkaConsumerGroup:      envOrDefault("KAFKA_CONSUMER_GROUP", "osac-metering-cost-management"),
		KafkaTopicPrefix:        envOrDefault("KAFKA_TOPIC_PREFIX", "osac.metering"),
		KafkaMode:               envOrDefault("KAFKA_MODE", defaultKafkaMode),
		KafkaReaderMode:         readerMode,
		KafkaHeartbeatPrefix:    envOrDefault("KAFKA_HEARTBEAT_PREFIX", "osac.metering.heartbeat."),
		KafkaIncludeInference:   os.Getenv("KAFKA_INCLUDE_INFERENCE") == "true",
		KafkaTLSEnabled:         kafkaTLSEnabled,
		KafkaTLSCACert:          kafkaTLSCACert,
		KafkaSASLUsername:       kafkaSASLUser,
		KafkaSASLPasswordFile:   os.Getenv("KAFKA_SASL_PASSWORD_FILE"),
		KafkaSASLPassword:       os.Getenv("KAFKA_SASL_PASSWORD"),
	}
}

func parseKafkaTLSEnabled(raw string, autoEnable bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return autoEnable
	}
}

func (c *Config) ComponentDisabled(name string) bool {
	return c.DisabledComponents[name]
}

func (c *Config) ReconcileEntityEnabled(name string) bool {
	if len(c.ReconcileEntities) == 0 || c.ReconcileEntities["all"] || c.ReconcileEntities["*"] {
		return true
	}
	return c.ReconcileEntities[strings.ToLower(name)]
}

func parseReconcileEntities(s string) map[string]bool {
	m := make(map[string]bool)
	if s == "" {
		return m
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			m[part] = true
		}
	}
	return m
}

func parseDisabledComponents(s string) map[string]bool {
	m := make(map[string]bool)
	if s == "" {
		return m
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			m[part] = true
		}
	}
	return m
}

func (c *Config) Validate() error {
	osacNeeded := !c.ComponentDisabled("watcher") || !c.ComponentDisabled("reconciler")
	if osacNeeded && c.OSACBaseURL == "" {
		return fmt.Errorf("OSAC_BASE_URL is required (or disable watcher+reconciler via DISABLE_COMPONENTS)")
	}
	if c.InventoryDBURL == "" {
		return fmt.Errorf("INVENTORY_DB_URL is required")
	}
	if c.KafkaReaderMode != "" && c.KafkaReaderMode != "legacy" && c.KafkaReaderMode != "osac-reader" {
		return fmt.Errorf("KAFKA_READER_MODE must be \"legacy\" or \"osac-reader\", got %q", c.KafkaReaderMode)
	}
	if c.KafkaBrokers != "" && c.KafkaSASLUsername != "" && c.KafkaSASLPasswordFile == "" && c.KafkaSASLPassword == "" {
		return fmt.Errorf("KAFKA_SASL_PASSWORD_FILE or KAFKA_SASL_PASSWORD is required when KAFKA_SASL_USERNAME is set")
	}
	return nil
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func intOrDefault(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}

func durationOrDefault(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return defaultVal
}
