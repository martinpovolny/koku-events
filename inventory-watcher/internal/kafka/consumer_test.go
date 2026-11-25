package kafka

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/osac-project/cost-event-consumer/internal/inventory"
)

type stubPermanentError struct {
	reason string
	msg    string
}

func (e stubPermanentError) Error() string            { return e.msg }
func (e stubPermanentError) DeadLetterReason() string { return e.reason }

type funcProcessor func(ctx context.Context, topic string, payload []byte) error

func (f funcProcessor) ProcessKafkaEvent(ctx context.Context, topic string, payload []byte) error {
	return f(ctx, topic, payload)
}

type recordingDLQ struct {
	records []inventory.KafkaDeadLetter
	failFor int
	calls   int
}

func (d *recordingDLQ) InsertKafkaDeadLetter(_ context.Context, dl inventory.KafkaDeadLetter) error {
	d.calls++
	if d.calls <= d.failFor {
		return errors.New("db temporarily unavailable")
	}
	d.records = append(d.records, dl)
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func writeTestCACert(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-kafka-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ca.crt")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatalf("write ca.crt: %v", err)
	}
	return path
}

func TestOSACReaderTopicPatterns(t *testing.T) {
	cfg := Config{
		TopicPrefix:      "osac.metering",
		ReaderMode:       ReaderModeOSACReader,
		IncludeInference: true,
	}
	include, exclude := cfg.OSACReaderTopicPatterns()
	if len(include) != 2 {
		t.Fatalf("expected 2 include patterns, got %v", include)
	}
	if len(exclude) != 1 {
		t.Fatalf("expected 1 exclude pattern, got %v", exclude)
	}

	incHB := regexp.MustCompile(include[0])
	incInf := regexp.MustCompile(include[1])
	exc := regexp.MustCompile(exclude[0])

	if !incHB.MatchString("osac.metering.heartbeat.tenant-acme") {
		t.Error("expected heartbeat pattern to match tenant-acme")
	}
	if incHB.MatchString("osac.metering.heartbeat.") {
		t.Error("expected heartbeat pattern not to match empty tenant suffix")
	}
	if !incInf.MatchString("osac.metering.inference") {
		t.Error("expected inference pattern to match osac.metering.inference")
	}
	if !exc.MatchString("osac.metering.heartbeat.system") {
		t.Error("expected exclude pattern to match system")
	}
	if !exc.MatchString("osac.metering.heartbeat.shared") {
		t.Error("expected exclude pattern to match shared")
	}
	if exc.MatchString("osac.metering.heartbeat.system-prod") {
		t.Error("expected exclude pattern NOT to match system-prod")
	}
}

func TestNewConsumer_OSACReaderTLSAndSCRAM(t *testing.T) {
	caPath := writeTestCACert(t)
	passPath := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passPath, []byte("super-secret-scram-pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Brokers:          []string{"kafka-external.apps.cluster-a.example.com:443"},
		ConsumerGroup:    "osac-metering-cost-management",
		TopicPrefix:      "osac.metering",
		ReaderMode:       ReaderModeOSACReader,
		IncludeInference: true,
		TLSEnabled:       true,
		TLSCACert:        caPath,
		SASLUsername:     "cost-management-reader",
		SASLPasswordFile: passPath,
	}

	c, err := NewConsumer(cfg, funcProcessor(func(context.Context, string, []byte) error { return nil }), testLogger())
	if err != nil {
		t.Fatalf("NewConsumer with TLS+SCRAM+regex failed: %v", err)
	}
	defer c.Close()

	pass, err := cfg.resolveSASLPassword()
	if err != nil || pass != "super-secret-scram-pass" {
		t.Fatalf("resolveSASLPassword = %q, %v", pass, err)
	}
}

func TestBuildClientOptions_RejectsBadCAAndMissingPassword(t *testing.T) {
	badCAPath := filepath.Join(t.TempDir(), "bad.crt")
	if err := os.WriteFile(badCAPath, []byte("not-a-cert"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := (Config{
		Brokers:    []string{"localhost:9093"},
		TLSEnabled: true,
		TLSCACert:  badCAPath,
	}).BuildClientOptions(); err == nil {
		t.Error("expected invalid CA PEM to fail")
	}

	if _, err := (Config{
		Brokers:      []string{"localhost:9093"},
		SASLUsername: "reader",
	}).BuildClientOptions(); err == nil {
		t.Error("expected missing SASL password to fail")
	}
}

func TestProcessPartitions_PermanentErrorDeadLettersAndCommits(t *testing.T) {
	dlq := &recordingDLQ{}
	var committed []*kgo.Record

	c := &Consumer{
		processor: funcProcessor(func(_ context.Context, _ string, payload []byte) error {
			if string(payload) == "poison" {
				return stubPermanentError{reason: "validation_error", msg: "bad schema"}
			}
			return nil
		}),
		deadLetters: dlq,
		logger:      testLogger(),
		commitRecords: func(_ context.Context, rs ...*kgo.Record) error {
			committed = append(committed, rs...)
			return nil
		},
		retryInitialBackoff: time.Millisecond,
		retryMaxBackoff:     2 * time.Millisecond,
	}

	recs := []*kgo.Record{
		{Topic: "osac.metering.heartbeat.acme", Partition: 0, Offset: 10, Key: []byte("k1"), Value: []byte("ok1")},
		{Topic: "osac.metering.heartbeat.acme", Partition: 0, Offset: 11, Key: []byte("k2"), Value: []byte("poison")},
		{Topic: "osac.metering.heartbeat.acme", Partition: 0, Offset: 12, Key: []byte("k3"), Value: []byte("ok2")},
	}

	err := c.processPartitions(context.Background(), []kgo.FetchTopicPartition{{
		Topic:          "osac.metering.heartbeat.acme",
		FetchPartition: kgo.FetchPartition{Partition: 0, Records: recs},
	}})
	if err != nil {
		t.Fatalf("processPartitions failed: %v", err)
	}
	if len(dlq.records) != 1 {
		t.Fatalf("dead letters = %d, want 1", len(dlq.records))
	}
	if dlq.records[0].RecordOffset != 11 || dlq.records[0].Reason != "validation_error" || string(dlq.records[0].Payload) != "poison" {
		t.Errorf("unexpected dead letter record: %+v", dlq.records[0])
	}
	if len(committed) != 3 {
		t.Fatalf("committed records = %d, want 3", len(committed))
	}
}

func TestProcessPartitions_TransientErrorRetriesWithoutCommittingFailedRecord(t *testing.T) {
	dlq := &recordingDLQ{}
	var committed []*kgo.Record
	var attempts atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := &Consumer{
		processor: funcProcessor(func(_ context.Context, _ string, payload []byte) error {
			if string(payload) == "transient" {
				if attempts.Add(1) >= 2 {
					cancel()
				}
				return errors.New("connection refused")
			}
			return nil
		}),
		deadLetters: dlq,
		logger:      testLogger(),
		commitRecords: func(_ context.Context, rs ...*kgo.Record) error {
			committed = append(committed, rs...)
			return nil
		},
		retryInitialBackoff: time.Millisecond,
		retryMaxBackoff:     2 * time.Millisecond,
	}

	recs := []*kgo.Record{
		{Topic: "osac.metering.heartbeat.acme", Partition: 0, Offset: 10, Value: []byte("ok1")},
		{Topic: "osac.metering.heartbeat.acme", Partition: 0, Offset: 11, Value: []byte("transient")},
		{Topic: "osac.metering.heartbeat.acme", Partition: 0, Offset: 12, Value: []byte("ok2")},
	}

	err := c.processPartitions(ctx, []kgo.FetchTopicPartition{{
		Topic:          "osac.metering.heartbeat.acme",
		FetchPartition: kgo.FetchPartition{Partition: 0, Records: recs},
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if len(dlq.records) != 0 {
		t.Fatalf("transient error must not dead-letter, got %d dead letters", len(dlq.records))
	}
	if len(committed) != 1 || committed[0].Offset != 10 {
		t.Fatalf("expected only offset 10 committed before transient failure, got %+v", committed)
	}
}

func TestProcessPartitions_DLQFailureRetriesUntilPersisted(t *testing.T) {
	dlq := &recordingDLQ{failFor: 1}
	var committed []*kgo.Record

	c := &Consumer{
		processor: funcProcessor(func(_ context.Context, _ string, _ []byte) error {
			return stubPermanentError{reason: "invalid_json", msg: "syntax error"}
		}),
		deadLetters: dlq,
		logger:      testLogger(),
		commitRecords: func(_ context.Context, rs ...*kgo.Record) error {
			committed = append(committed, rs...)
			return nil
		},
		retryInitialBackoff: time.Millisecond,
		retryMaxBackoff:     2 * time.Millisecond,
	}

	recs := []*kgo.Record{
		{Topic: "osac.metering.heartbeat.acme", Partition: 0, Offset: 5, Value: []byte("{bad")},
	}
	if err := c.processPartitions(context.Background(), []kgo.FetchTopicPartition{{
		Topic:          "osac.metering.heartbeat.acme",
		FetchPartition: kgo.FetchPartition{Partition: 0, Records: recs},
	}}); err != nil {
		t.Fatalf("expected recovery after transient DLQ error, got %v", err)
	}
	if dlq.calls != 2 || len(dlq.records) != 1 {
		t.Fatalf("expected 2 DLQ attempts and 1 stored record, got calls=%d stored=%d", dlq.calls, len(dlq.records))
	}
	if len(committed) != 1 || committed[0].Offset != 5 {
		t.Fatalf("expected offset 5 committed only after DLQ persisted, got %+v", committed)
	}
}
