package provider

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

func TestCachedGet(t *testing.T) {
	client := &anthropic.Client{}
	var calls atomic.Int32
	list := func(context.Context) (map[string]string, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("list failed")
		}
		return map[string]string{"a": "alpha"}, nil
	}

	if _, err := cachedGet(t.Context(), client, "scope", "a", list); err == nil {
		t.Error("a failed list must return its error")
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
	if calls.Load() != 2 {
		t.Errorf("expected a failed list to be retried once, got %d calls", calls.Load())
	}

	if item, err := cachedGet(t.Context(), client, "scope", "missing", list); err != nil || item != nil {
		t.Errorf("missing item must be nil without error, got %v, %v", item, err)
	}

	invalidateList(client, "scope")
	if _, err := cachedGet(t.Context(), client, "scope", "a", list); err != nil || calls.Load() != 3 {
		t.Errorf("invalidate must force a new list call, got %d calls, err %v", calls.Load(), err)
	}
}
