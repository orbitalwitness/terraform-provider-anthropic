package provider

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestCachedGet(t *testing.T) {
	client := &anthropic.Client{}
	var calls atomic.Int32
	list := func(context.Context) (map[string]string, error) {
		calls.Add(1)
		return map[string]string{"a": "alpha"}, nil
	}

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			item, err := cachedGet(t.Context(), client, "scope", "a", list)
			if err != nil || item == nil || *item != "alpha" {
				t.Errorf("got %v, %v", item, err)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Errorf("expected one list call, got %d", calls.Load())
	}

	if item, err := cachedGet(t.Context(), client, "scope", "missing", list); err != nil || item != nil {
		t.Errorf("missing item must be nil without error, got %v, %v", item, err)
	}

	invalidateList(client, "scope")
	if _, err := cachedGet(t.Context(), client, "scope", "a", list); err != nil || calls.Load() != 2 {
		t.Errorf("invalidate must force a new list call, got %d calls, err %v", calls.Load(), err)
	}
}
