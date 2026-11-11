package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/usagecontent"
	"github.com/stretchr/testify/require"
)

func TestUsageContentBillingContextCarriesImmutableSession(t *testing.T) {
	// A session with no payload/manager suffices to test context propagation:
	// it is never finalized or written by this context-only test.
	s := &usagecontent.Session{}
	parent := usagecontent.WithSession(context.Background(), s)
	child := usageRecordContext(parent, context.Background())
	require.Same(t, s, usagecontent.FromContext(child))
	task, abandon := wrapUsageRecordTaskContext(parent, func(ctx context.Context) { require.Same(t, s, usagecontent.FromContext(ctx)) })
	task(context.Background())
	abandon() // Also exercises double-release safety after an executed task.
}
