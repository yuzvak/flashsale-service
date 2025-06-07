package monitoring

import (
	"context"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisHook struct{}

func (RedisHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, "start_time", time.Now()), nil
}

func (RedisHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	start, ok := ctx.Value("start_time").(time.Time)
	if !ok {
		return nil
	}

	duration := time.Since(start).Seconds()
	RedisCommandDuration.WithLabelValues(cmd.Name()).Observe(duration)
	return nil
}

func (RedisHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, "pipeline_start_time", time.Now()), nil
}

func (RedisHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	start, ok := ctx.Value("pipeline_start_time").(time.Time)
	if !ok {
		return nil
	}

	duration := time.Since(start).Seconds()
	RedisCommandDuration.WithLabelValues("pipeline").Observe(duration)
	return nil
}

func (RedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		duration := time.Since(start).Seconds()
		RedisCommandDuration.WithLabelValues(cmd.Name()).Observe(duration)
		return err
	}
}

func (RedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		duration := time.Since(start).Seconds()
		RedisCommandDuration.WithLabelValues("pipeline").Observe(duration)
		return err
	}
}

func (RedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		start := time.Now()
		conn, err := next(ctx, network, addr)
		duration := time.Since(start).Seconds()
		RedisCommandDuration.WithLabelValues("dial").Observe(duration)
		return conn, err
	}
}

func InstrumentRedisClient(client *redis.Client) *redis.Client {
	client.AddHook(&RedisHook{})
	return client
}

type BloomFilterMetrics struct {
	filterName string
}

func NewBloomFilterMetrics(filterName string) *BloomFilterMetrics {
	return &BloomFilterMetrics{
		filterName: filterName,
	}
}

type DistributedLockMetrics struct {
	lockKey string
}

func NewDistributedLockMetrics(lockKey string) *DistributedLockMetrics {
	return &DistributedLockMetrics{
		lockKey: lockKey,
	}
}

func (m *DistributedLockMetrics) RecordAttempt() {
	RecordLockAttempt(m.lockKey)
}

func (m *DistributedLockMetrics) RecordSuccess() {
	RecordLockSuccess(m.lockKey)
}

func (m *DistributedLockMetrics) RecordFailure(reason string) {
	RecordLockFailure(m.lockKey, reason)
}

func (m *DistributedLockMetrics) TimeOperation() func() {
	return TimeRedisLock(m.lockKey)
}
