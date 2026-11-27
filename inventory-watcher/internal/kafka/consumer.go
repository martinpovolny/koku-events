package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/osac-project/cost-event-consumer/internal/inventory"
	"github.com/osac-project/cost-event-consumer/internal/metrics"
)

const (
	defaultRetryInitialBackoff = 250 * time.Millisecond
	defaultRetryMaxBackoff     = 5 * time.Second
	defaultOSACMetadataMaxAge  = 30 * time.Second
	commitTimeout              = 5 * time.Second
)

// EventProcessor is implemented by the component that handles incoming
// Kafka records (typically the API handler). Using an interface here
// avoids circular imports between the kafka and api packages.
type EventProcessor interface {
	ProcessKafkaEvent(ctx context.Context, eventType string, payload []byte) error
}

// PermanentError is implemented by processing errors that cannot succeed on
// retry (invalid JSON, schema/contract validation errors, unsupported event
// versions, receipt identity collisions). The consumer durably persists these
// records to kafka_dead_letters before advancing the offset.
type PermanentError interface {
	error
	DeadLetterReason() string
}

// DeadLetterWriter persists permanently failed Kafka records to durable
// storage (implemented by *inventory.Store).
type DeadLetterWriter interface {
	InsertKafkaDeadLetter(ctx context.Context, dl inventory.KafkaDeadLetter) error
}

// ConsumerOption configures optional Consumer dependencies.
type ConsumerOption func(*Consumer)

// WithDeadLetterWriter configures durable PostgreSQL dead-letter storage for
// records that fail with permanent errors.
func WithDeadLetterWriter(w DeadLetterWriter) ConsumerOption {
	return func(c *Consumer) {
		c.deadLetters = w
	}
}

// Consumer reads CloudEvent records from Kafka and feeds them into the
// existing processing pipeline via the EventProcessor interface.
type Consumer struct {
	client              *kgo.Client
	processor           EventProcessor
	deadLetters         DeadLetterWriter
	logger              *slog.Logger
	commitRecords       func(context.Context, ...*kgo.Record) error
	retryInitialBackoff time.Duration
	retryMaxBackoff     time.Duration
}

// NewConsumer creates a franz-go client configured for group consumption with
// manual offset commits, optional TLS + SCRAM-SHA-512 authentication, and
// either legacy 3-topic or OSAC metering.readers dynamic regex subscription.
func NewConsumer(cfg Config, processor EventProcessor, logger *slog.Logger, opts ...ConsumerOption) (*Consumer, error) {
	kgoOpts, err := cfg.BuildClientOptions()
	if err != nil {
		return nil, fmt.Errorf("kafka consumer: %w", err)
	}

	mode := cfg.NormalizedReaderMode()
	kgoOpts = append(kgoOpts,
		kgo.ConsumerGroup(cfg.ConsumerGroup),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
	)

	var topicDesc any
	if mode == ReaderModeOSACReader {
		include, exclude := cfg.OSACReaderTopicPatterns()
		kgoOpts = append(kgoOpts,
			kgo.ConsumeRegex(),
			kgo.ConsumeTopics(include...),
			kgo.ConsumeExcludeTopics(exclude...),
			kgo.MetadataMaxAge(defaultOSACMetadataMaxAge),
		)
		topicDesc = map[string][]string{"include": include, "exclude": exclude}
	} else {
		topics := cfg.Topics()
		kgoOpts = append(kgoOpts, kgo.ConsumeTopics(topics...))
		topicDesc = topics
	}

	cl, err := kgo.NewClient(kgoOpts...)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer: %w", err)
	}

	c := &Consumer{
		client:              cl,
		processor:           processor,
		logger:              logger,
		commitRecords:       cl.CommitRecords,
		retryInitialBackoff: defaultRetryInitialBackoff,
		retryMaxBackoff:     defaultRetryMaxBackoff,
	}
	for _, opt := range opts {
		opt(c)
	}

	logger.Info("kafka consumer created",
		"brokers", cleanBrokers(cfg.Brokers),
		"group", cfg.ConsumerGroup,
		"reader_mode", mode,
		"tls", cfg.TLSEnabled,
		"sasl", cfg.SASLUsername != "",
		"topics", topicDesc,
	)
	return c, nil
}

// Run polls Kafka in a loop until ctx is cancelled. Records are processed in
// partition offset order. Offsets are committed strictly after DB processing
// succeeds or a permanent failure is durably persisted to kafka_dead_letters.
// Transient errors are retried with bounded exponential backoff without
// advancing offsets.
func (c *Consumer) Run(ctx context.Context) error {
	c.logger.Info("kafka consumer loop starting")
	for {
		fetches := c.client.PollFetches(ctx)

		if ctx.Err() != nil {
			c.logger.Info("kafka consumer: context cancelled, stopping loop")
			return ctx.Err()
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				c.logger.Error("kafka fetch error",
					"topic", e.Topic,
					"partition", e.Partition,
					"err", e.Err,
				)
			}
		}

		var partitions []kgo.FetchTopicPartition
		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			if len(p.Records) > 0 {
				partitions = append(partitions, p)
			}
		})
		if len(partitions) == 0 {
			continue
		}

		if err := c.processPartitions(ctx, partitions); err != nil {
			if ctx.Err() != nil {
				c.logger.Info("kafka consumer: context cancelled during processing")
				return ctx.Err()
			}
			c.logger.Error("kafka consumer: batch processing error", "err", err)
		}
	}
}

// processPartitions processes fetched records per partition in offset order
// and commits only records that succeeded or were durably dead-lettered.
func (c *Consumer) processPartitions(ctx context.Context, partitions []kgo.FetchTopicPartition) error {
	var committed []*kgo.Record

	flushCommits := func() {
		if len(committed) == 0 || c.commitRecords == nil {
			return
		}
		commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), commitTimeout)
		defer cancel()
		if err := c.commitRecords(commitCtx, committed...); err != nil {
			c.logger.Error("kafka consumer: offset commit failed", "records", len(committed), "err", err)
		}
		committed = nil
	}

	for _, p := range partitions {
		for _, r := range p.Records {
			if err := c.handleRecordWithRetry(ctx, r, flushCommits); err != nil {
				flushCommits()
				return err
			}
			committed = append(committed, r)
		}
	}

	flushCommits()
	return nil
}

func (c *Consumer) handleRecordWithRetry(ctx context.Context, r *kgo.Record, flushBeforeRetry func()) error {
	backoff := c.retryInitialBackoff
	if backoff <= 0 {
		backoff = defaultRetryInitialBackoff
	}
	maxBackoff := c.retryMaxBackoff
	if maxBackoff < backoff {
		maxBackoff = backoff
	}

	for {
		err := c.processor.ProcessKafkaEvent(ctx, r.Topic, r.Value)
		if err == nil {
			return nil
		}

		var perm PermanentError
		if errors.As(err, &perm) {
			reason := perm.DeadLetterReason()
			if reason == "" {
				reason = "permanent_error"
			}
			c.logger.Error("kafka: permanent event processing failure",
				"topic", r.Topic,
				"partition", r.Partition,
				"offset", r.Offset,
				"key", string(r.Key),
				"reason", reason,
				"err", err,
			)
			if c.deadLetters != nil {
				dlErr := c.deadLetters.InsertKafkaDeadLetter(ctx, inventory.KafkaDeadLetter{
					Topic:        r.Topic,
					Partition:    r.Partition,
					RecordOffset: r.Offset,
					RecordKey:    r.Key,
					Payload:      r.Value,
					Reason:       reason,
					ErrorMessage: err.Error(),
				})
				if dlErr != nil {
					c.logger.Error("kafka: failed to persist dead letter, retrying without committing offset",
						"topic", r.Topic,
						"partition", r.Partition,
						"offset", r.Offset,
						"err", dlErr,
					)
					flushBeforeRetry()
					if waitErr := waitBackoff(ctx, backoff); waitErr != nil {
						return waitErr
					}
					backoff = nextBackoff(backoff, maxBackoff)
					continue
				}
			}
			metrics.KafkaDeadLettersTotal.WithLabelValues(r.Topic, reason).Inc()
			return nil
		}

		c.logger.Error("kafka: transient event processing failure, retrying without committing offset",
			"topic", r.Topic,
			"partition", r.Partition,
			"offset", r.Offset,
			"key", string(r.Key),
			"backoff", backoff,
			"err", err,
		)
		flushBeforeRetry()
		if waitErr := waitBackoff(ctx, backoff); waitErr != nil {
			return waitErr
		}
		backoff = nextBackoff(backoff, maxBackoff)
	}
}

func waitBackoff(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

// Close shuts down the consumer client, leaving the consumer group
// so that partitions are rebalanced promptly.
func (c *Consumer) Close() {
	c.client.Close()
	c.logger.Info("kafka consumer closed")
}
