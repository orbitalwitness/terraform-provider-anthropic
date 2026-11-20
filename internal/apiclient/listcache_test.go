package apiclient

import (
	"sync"
	"sync/atomic"
	"testing"
)

type slicePager struct {
	items []string
	cur   string
}

func (p *slicePager) Next() bool {
	if len(p.items) == 0 {
		return false
	}
	p.cur, p.items = p.items[0], p.items[1:]
	return true
}

func (p *slicePager) Current() string { return p.cur }
func (p *slicePager) Err() error      { return nil }

func TestListCache(t *testing.T) {
	cache := NewListCache(func(v string) string { return v })
	var lists atomic.Int32
	list := func() Pager[string] {
		lists.Add(1)
		return &slicePager{items: []string{"a", "b", "c"}}
	}

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			item, err := cache.Get("org", "b", list)
			if err != nil || *item != "b" {
				t.Errorf("got %v, %v", item, err)
			}
		})
	}
	wg.Wait()
	if n := lists.Load(); n != 1 {
		t.Fatalf("expected 1 list call, got %d", n)
	}

	if _, err := cache.Get("org", "missing", list); !IsNotFoundError(err) {
		t.Fatalf("expected not found, got %v", err)
	}

	cache.Reset()
	if _, err := cache.Get("org", "a", list); err != nil || lists.Load() != 2 {
		t.Fatalf("expected relist after reset, got %v, %d lists", err, lists.Load())
	}
}
