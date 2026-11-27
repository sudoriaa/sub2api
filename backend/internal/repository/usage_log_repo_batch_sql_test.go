//go:build unit

package repository

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func batchSQLTestData(widths []int) ([]string, map[string]usageLogInsertPrepared, []usageLogInsertPrepared) {
	keys := make([]string, len(widths))
	byKey := make(map[string]usageLogInsertPrepared, len(widths))
	rows := make([]usageLogInsertPrepared, len(widths))
	for i, w := range widths {
		keys[i] = strconv.Itoa(len(widths) - i)
		row := make([]any, w)
		for j := range row {
			row[j] = i*1000 + j + 100
		}
		rows[i].args = row
		byKey[keys[i]] = rows[i]
	}
	return keys, byKey, rows
}

func TestUsageLogBatchSQLParity(t *testing.T) {
	// SHA-256 of the exact strings emitted by the two original builders.
	// In particular, a nonstandard shape with the same row count as an
	// already-cached query must retain every old cast/placeholder/whitespace.
	cases := []struct {
		widths              []int
		indexed, bestEffort string
	}{
		{nil, "15b1402c7b97f8d1da446f9b5f39f43559c7c6110bbbe1c70a58c2de82f387db", "6592d9e17c93c92971c667714687c7d8444d09a9b8a73a0d771c1868d1a9f740"},
		{[]int{68}, "ebdd80c6bed5a8e6691622d46e62609eec64c1149e82c3c97b947beee729ca6b", "340058762ae3ee504cc5470c0e5bbbabc5e68315d01758a61af95a77c84ee5cd"},
		{[]int{68, 68, 68}, "0ff41ca3d95f6dd1ed9c2f46cb6f123dff0868aefdb7ce426accb45567817832", "fa32a8e58e36c5a86bc81140e173ee4d59343b09b6ee25ec5c7889d47117c4c1"},
		{[]int{0, 3, 69}, "cb51931bf8b584ebf93d921ff26afcdbd7d0849146d9d7adf0c57abb5b17f8eb", "293513717e3d2d9e0b1839fd609abb9c3dc412053523229226fdb81b1ed6830d"},
		{[]int{3, 0, 68}, "22afb7735de290df82e0cc339b0af52cbd9e7db9563abaafb35ff9fcd5ae377c", "ce9fa4d76736c31ebf925d86eaa80c8151348bd564fe107308bf96d5429e59c7"},
	}
	for _, tc := range cases {
		keys, byKey, rows := batchSQLTestData(tc.widths)
		for _, indexed := range []bool{true, false} {
			want := make([]any, 0)
			for i, row := range rows {
				if indexed {
					want = append(want, i)
				}
				want = append(want, row.args...)
			}
			for range 2 {
				var q string
				var args []any
				hash := tc.bestEffort
				if indexed {
					q, args = buildUsageLogBatchInsertQuery(keys, byKey)
					hash = tc.indexed
				} else {
					q, args = buildUsageLogBestEffortInsertQuery(rows)
				}
				require.Equal(t, hash, fmt.Sprintf("%x", sha256.Sum256([]byte(q))))
				require.Equal(t, want, args)
				if len(args) > 0 {
					args[0] = "must not affect a second call"
				}
			}
		}
	}
}

func TestUsageLogBatchSQLConcurrent(t *testing.T) {
	widths := slices.Repeat([]int{len(usageLogInsertArgTypes)}, 17)
	keys, byKey, rows := batchSQLTestData(widths)
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			for range 25 {
				indexed, ia := buildUsageLogBatchInsertQuery(keys, byKey)
				plain, pa := buildUsageLogBestEffortInsertQuery(rows)
				if indexed == plain || ia[0] != 0 || pa[0] != 100 || len(ia) != cap(ia) || len(pa) != cap(pa) {
					t.Error("invalid cached batch")
					return
				}
				ia[0], pa[0] = nil, nil // shared args would race with other readers
			}
		})
	}
	wg.Wait()
}

func BenchmarkUsageLogBatchSQL(b *testing.B) {
	keys, byKey, rows := batchSQLTestData(slices.Repeat([]int{len(usageLogInsertArgTypes)}, 64))
	for _, indexed := range []bool{true, false} {
		name := "BestEffort"
		if indexed {
			name = "Indexed"
		}
		b.Run(name, func(b *testing.B) {
			var q string
			var args []any
			for b.Loop() {
				if indexed {
					q, args = buildUsageLogBatchInsertQuery(keys, byKey)
				} else {
					q, args = buildUsageLogBestEffortInsertQuery(rows)
				}
			}
			if len(q) == 0 || len(args) < len(rows)*len(usageLogInsertArgTypes) {
				b.Fatal("incomplete batch")
			}
		})
	}
}
