package bloom

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"hash/fnv"
	"math"

	"github.com/redis/go-redis/v9"
)

type RedisBloomFilter struct {
	client *redis.Client
	key    string
	m      uint64 // size in bits
	k      uint64 // number of hash functions
}

func NewRedisBloomFilter(client *redis.Client, key string, m, k uint64) *RedisBloomFilter {
	return &RedisBloomFilter{
		client: client,
		key:    key,
		m:      m,
		k:      k,
	}
}

func (bf *RedisBloomFilter) Add(ctx context.Context, element string) error {
	hashes := bf.getHashes(element)

	pipe := bf.client.Pipeline()

	for i := uint64(0); i < bf.k; i++ {
		bitPos := hashes[i] % bf.m
		pipe.SetBit(ctx, bf.key, int64(bitPos), 1)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (bf *RedisBloomFilter) Contains(ctx context.Context, element string) (bool, error) {
	hashes := bf.getHashes(element)

	pipe := bf.client.Pipeline()
	cmds := make([]*redis.IntCmd, bf.k)

	for i := uint64(0); i < bf.k; i++ {
		bitPos := hashes[i] % bf.m
		cmds[i] = pipe.GetBit(ctx, bf.key, int64(bitPos))
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	for _, cmd := range cmds {
		if cmd.Val() == 0 {
			return false, nil
		}
	}

	return true, nil
}

func (bf *RedisBloomFilter) Clear(ctx context.Context) error {
	return bf.client.Del(ctx, bf.key).Err()
}

func (bf *RedisBloomFilter) getHashes(element string) []uint64 {
	hashes := make([]uint64, bf.k)

	h1 := bf.hash1(element)
	h2 := bf.hash2(element)

	for i := uint64(0); i < bf.k; i++ {
		hashes[i] = h1 + i*h2
	}

	return hashes
}

func (bf *RedisBloomFilter) hash1(element string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(element))
	return h.Sum64()
}

func (bf *RedisBloomFilter) hash2(element string) uint64 {
	h := sha256.Sum256([]byte(element))
	return binary.BigEndian.Uint64(h[:8])
}

func (bf *RedisBloomFilter) EstimateFalsePositiveRate(elementsAdded uint64) float64 {
	if elementsAdded == 0 {
		return 0.0
	}

	exponent := -float64(bf.k*elementsAdded) / float64(bf.m)
	base := 1.0 - math.Exp(exponent)
	return math.Pow(base, float64(bf.k))
}

func GetOptimalParameters(expectedElements uint64, falsePositiveRate float64) (m, k uint64) {

	mFloat := -float64(expectedElements) * math.Log(falsePositiveRate) / (math.Log(2) * math.Log(2))
	m = uint64(math.Ceil(mFloat))

	kFloat := (float64(m) / float64(expectedElements)) * math.Log(2)
	k = uint64(math.Round(kFloat))

	if k == 0 {
		k = 1
	}

	return m, k
}
