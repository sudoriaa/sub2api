//go:build unit

package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/regional"
	"github.com/stretchr/testify/require"
)

type regionalSnapshotFeed struct {
	current atomic.Pointer[regional.Snapshot]
}

func (f *regionalSnapshotFeed) Current() (regional.Snapshot, error) {
	if s := f.current.Load(); s != nil {
		return *s, nil
	}
	return regional.Snapshot{}, regional.ErrSyncUnavailable
}

func validationTestSnapshot(tb testing.TB, rev uint64, name string, count int, conflict bool, expiry *time.Time) *regional.Snapshot {
	tb.Helper()
	rows := make([]*Account, count)
	for i := range rows {
		rows[i] = &Account{ID: int64(i + 1), Name: name, Platform: PlatformOpenAI, GroupIDs: []int64{10}, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"model": name}, AutoPauseOnExpired: expiry != nil, ExpiresAt: expiry}
	}
	doc := RegionalSchedulerDocument{Version: 1, Groups: []*Group{{ID: 10, Name: name, Status: StatusActive}}, Buckets: []RegionalSchedulerBucket{
		{Bucket: SchedulerBucket{GroupID: 10, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}, Accounts: rows},
		{Bucket: SchedulerBucket{GroupID: 10, Platform: PlatformOpenAI, Mode: SchedulerModeForced}, Accounts: append([]*Account(nil), rows...)},
	}}
	if conflict {
		last := *rows[0]
		last.Name = "conflict"
		doc.Buckets[1].Accounts[0] = &last
	}
	payload, err := json.Marshal(doc)
	require.NoError(tb, err)
	return &regional.Snapshot{Kind: regional.SnapshotScheduler, Revision: rev, PayloadSHA256: fmt.Sprintf("%x", sha256.Sum256(payload)), Payload: payload, ExpiresAt: time.Now().Add(time.Hour)}
}

func TestRegionalSchedulerReaderConcurrentRevisionSwaps(t *testing.T) {
	feed := &regionalSnapshotFeed{}
	r := &RegionalSchedulerReader{replica: feed}
	a := validationTestSnapshot(t, 1, "first", 1, false, nil)
	b := validationTestSnapshot(t, 2, "second", 1, false, nil)
	bad := validationTestSnapshot(t, 2, "denied", 1, true, nil)
	feed.current.Store(a)
	ctx := t.Context()
	bucket := SchedulerBucket{GroupID: 10, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 60 {
				rows, hit, err := r.GetLocalSnapshot(ctx, bucket)
				if err != nil {
					if err.Error() != "conflicting regional account views" {
						t.Error(err)
						return
					}
					continue
				}
				if !hit || len(rows) != 1 || (rows[0].Name != "first" && rows[0].Name != "second") || rows[0].Credentials["model"] != rows[0].Name {
					t.Error("invalid or shared snapshot result")
					return
				}
				rows[0].Credentials["model"] = "request-owned"
			}
		})
	}
	for range 60 {
		for _, snapshot := range []*regional.Snapshot{b, bad, a} {
			feed.current.Store(snapshot)
			g, err := r.GetLocalGroup(ctx, 10)
			if snapshot == bad {
				if err == nil {
					t.Error("memo accepted a conflicting view with the same revision")
				}
			} else if err != nil || g.Name != map[uint64]string{1: "first", 2: "second"}[snapshot.Revision] {
				t.Errorf("wrong group for revision %d: %v", snapshot.Revision, err)
			} else {
				g.Name = "request-owned"
			}
		}
	}
	wg.Wait()
	// Failure is not a validation result; it must not displace a valid key.
	feed.current.Store(bad)
	_, err := r.GetLocalAccounts(ctx, []int64{1})
	require.EqualError(t, err, "conflicting regional account views")
	require.NotEqual(t, bad.PayloadSHA256, r.validated.Load().payloadSHA256)
	feed.current.Store(nil)
	_, err = r.GetLocalAccounts(ctx, []int64{1})
	require.ErrorIs(t, err, regional.ErrSyncUnavailable)
}

func TestRegionalSchedulerReaderWarmTimeGates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		expires := time.Now().Add(time.Second)
		snapshot := validationTestSnapshot(t, 1, "test", 1, false, &expires)
		snapshot.ExpiresAt = expires.Add(time.Second)
		feed := &regionalSnapshotFeed{}
		feed.current.Store(snapshot)
		r := &RegionalSchedulerReader{replica: feed}
		ctx := t.Context()
		rows, err := r.GetLocalAccounts(ctx, []int64{1})
		require.NoError(t, err)
		require.Contains(t, rows, int64(1))
		time.Sleep(time.Second)
		rows, err = r.GetLocalAccounts(ctx, []int64{1})
		require.NoError(t, err)
		require.Empty(t, rows, "same bytes, but account expired")
		time.Sleep(time.Second)
		_, err = r.GetLocalGroup(ctx, 10)
		require.ErrorIs(t, err, ErrLocalSchedulerSnapshotExpired)
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		_, err = r.GetLocalGroup(canceled, 10)
		require.True(t, errors.Is(err, context.Canceled))
	})
}

func BenchmarkRegionalSchedulerWarmValidation(b *testing.B) {
	s := validationTestSnapshot(b, 1, "bench", 64, false, nil)
	r := &RegionalSchedulerReader{}
	_, err := r.decodeSnapshot(*s)
	require.NoError(b, err)
	b.ReportAllocs()
	for b.Loop() {
		doc, err := r.decodeSnapshot(*s)
		if err != nil || len(doc.Buckets) != 2 {
			b.Fatal("invalid snapshot", err)
		}
	}
}
