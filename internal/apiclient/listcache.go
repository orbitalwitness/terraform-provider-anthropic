package apiclient

import (
	"errors"
	"sync"
)

var errNotInList = errors.New("not found in list")

type Pager[V any] interface {
	Next() bool
	Current() V
	Err() error
}

// ListCache serves single-item reads from one paginated List call per scope.
// Concurrent Gets for the same scope share a single List call.
type ListCache[V any] struct {
	key   func(V) string
	mu    sync.Mutex
	fills map[string]*listFill[V]
}

type listFill[V any] struct {
	once  sync.Once
	items map[string]V
	err   error
}

func NewListCache[V any](key func(V) string) *ListCache[V] {
	return &ListCache[V]{key: key, fills: map[string]*listFill[V]{}}
}

// Get returns the item with the given id in scope, listing the scope on first
// use. A missing item yields an error that IsNotFoundError recognises.
func (c *ListCache[V]) Get(scope, id string, list func() Pager[V]) (*V, error) {
	c.mu.Lock()
	f, ok := c.fills[scope]
	if !ok {
		f = &listFill[V]{}
		c.fills[scope] = f
	}
	c.mu.Unlock()

	f.once.Do(func() {
		pager := list()
		items := map[string]V{}
		for pager.Next() {
			items[c.key(pager.Current())] = pager.Current()
		}
		f.items, f.err = items, pager.Err()
	})
	if f.err != nil {
		return nil, f.err
	}
	item, ok := f.items[id]
	if !ok {
		return nil, errNotInList
	}
	return &item, nil
}

// Reset discards every cached scope so later reads list again.
func (c *ListCache[V]) Reset() {
	c.mu.Lock()
	c.fills = map[string]*listFill[V]{}
	c.mu.Unlock()
}
