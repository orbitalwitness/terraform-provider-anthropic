package provider

import (
	"context"
	"sync"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

// listCache lets each provider instance (one plan or apply) refresh high-cardinality resources with one paginated
// list per parent rather than one GET per resource. It is keyed by client because the framework builds a new
// resource for every request.
var listCache sync.Map // listCacheKey -> *listCacheEntry

type listCacheKey struct {
	client *anthropic.Client
	scope  string
}

type listCacheEntry struct {
	mu    sync.Mutex
	items any
}

func invalidateList(client *anthropic.Client, scope string) {
	listCache.Delete(listCacheKey{client, scope})
}

// cachedGet returns the item with the given ID from the list for scope, calling list at most once per scope until
// it succeeds or the scope is invalidated. A nil item means the ID is absent from the list.
func cachedGet[T any](ctx context.Context, client *anthropic.Client, scope, id string, list func(context.Context) (map[string]T, error)) (*T, error) {
	value, _ := listCache.LoadOrStore(listCacheKey{client, scope}, &listCacheEntry{})
	entry, _ := value.(*listCacheEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.items == nil {
		items, err := list(ctx)
		if err != nil {
			return nil, err
		}
		entry.items = items
	}
	item, ok := entry.items.(map[string]T)[id]
	if !ok {
		return nil, nil
	}
	return &item, nil
}
