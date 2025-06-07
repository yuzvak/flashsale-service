package bloom

import (
	"hash/fnv"
	"math"
	"sync"
)

type BloomFilter struct {
	bitset    []bool
	size      uint
	hashCount uint
	mutex     sync.RWMutex
}

func NewBloomFilter(size, hashCount uint) *BloomFilter {
	return &BloomFilter{
		bitset:    make([]bool, size),
		size:      size,
		hashCount: hashCount,
	}
}

func NewBloomFilterWithExpectedItems(expectedItems uint, falsePositiveProb float64) *BloomFilter {
	size := optimalSize(expectedItems, falsePositiveProb)
	hashCount := optimalHashCount(size, expectedItems)

	return NewBloomFilter(size, hashCount)
}

func (bf *BloomFilter) Add(item string) {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	for i := uint(0); i < bf.hashCount; i++ {
		position := bf.hash(item, i)
		bf.bitset[position] = true
	}
}

func (bf *BloomFilter) Contains(item string) bool {
	bf.mutex.RLock()
	defer bf.mutex.RUnlock()

	for i := uint(0); i < bf.hashCount; i++ {
		position := bf.hash(item, i)
		if !bf.bitset[position] {
			return false
		}
	}

	return true
}

func (bf *BloomFilter) Clear() {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	bf.bitset = make([]bool, bf.size)
}

func (bf *BloomFilter) hash(item string, seed uint) uint {
	h := fnv.New64a()
	h.Write([]byte(item))
	h.Write([]byte{byte(seed)})
	return uint(h.Sum64() % uint64(bf.size))
}

func optimalSize(expectedItems uint, falsePositiveProb float64) uint {
	return uint(math.Ceil(-float64(expectedItems) * math.Log(falsePositiveProb) / math.Pow(math.Log(2), 2)))
}

func optimalHashCount(size, expectedItems uint) uint {
	return uint(math.Max(1, math.Round(float64(size)/float64(expectedItems)*math.Log(2))))
}
