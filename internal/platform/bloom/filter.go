package bloom

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strings"
	"sync"
)

type Filter struct {
	mu        sync.RWMutex
	bits      []uint64
	bitCount  uint64
	hashCount uint64
}

func NewFilter(expectedItems uint64, falsePositiveRate float64) *Filter {
	if expectedItems == 0 {
		expectedItems = 1
	}
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		falsePositiveRate = 0.01
	}

	m := optimalBitCount(expectedItems, falsePositiveRate)
	k := optimalHashCount(expectedItems, m)
	if k == 0 {
		k = 1
	}

	return &Filter{
		bits:      make([]uint64, (m+63)/64),
		bitCount:  m,
		hashCount: k,
	}
}

func (f *Filter) Add(value string) {
	value = normalizeValue(value)
	if f == nil || value == "" {
		return
	}

	locations := f.locations(value)

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, location := range locations {
		word := location / 64
		bit := location % 64
		f.bits[word] |= 1 << bit
	}
}

func (f *Filter) Test(value string) bool {
	value = normalizeValue(value)
	if f == nil || value == "" {
		return false
	}

	locations := f.locations(value)

	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, location := range locations {
		word := location / 64
		bit := location % 64
		if f.bits[word]&(1<<bit) == 0 {
			return false
		}
	}

	return true
}

func (f *Filter) locations(value string) []uint64 {
	sum := sha256.Sum256([]byte(value))
	h1 := binary.LittleEndian.Uint64(sum[0:8])
	h2 := binary.LittleEndian.Uint64(sum[8:16])
	if h2 == 0 {
		h2 = 0x9e3779b97f4a7c15
	}

	locations := make([]uint64, 0, f.hashCount)
	for i := uint64(0); i < f.hashCount; i++ {
		locations = append(locations, (h1+i*h2)%f.bitCount)
	}

	return locations
}

func optimalBitCount(expectedItems uint64, falsePositiveRate float64) uint64 {
	m := -float64(expectedItems) * math.Log(falsePositiveRate) / (math.Ln2 * math.Ln2)
	if m < 64 {
		return 64
	}

	return uint64(math.Ceil(m))
}

func optimalHashCount(expectedItems, bitCount uint64) uint64 {
	if expectedItems == 0 || bitCount == 0 {
		return 1
	}

	k := (float64(bitCount) / float64(expectedItems)) * math.Ln2
	if k < 1 {
		return 1
	}

	return uint64(math.Ceil(k))
}

func normalizeValue(value string) string {
	return strings.TrimSpace(strings.ToUpper(value))
}
