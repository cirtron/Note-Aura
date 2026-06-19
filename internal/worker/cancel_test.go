package worker

import (
	"context"
	"testing"
	"time"
)

func TestCancelRegistry(t *testing.T) {
	w := &Worker{cancels: map[int64]context.CancelFunc{}}

	ctx, cancel := context.WithCancel(context.Background())
	w.register(42, cancel)

	w.Cancel(42) // should cancel ctx
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("Cancel did not cancel the registered context")
	}

	w.deregister(42)
	w.Cancel(42) // no-op, must not panic
}
