package cache

import (
	"sync"

	"github.com/octopipe/circlerr/internal/resource"
)

type localCache struct {
	mu sync.RWMutex

	cache          map[string]resource.Resource
	managedObjects map[string]resource.ManagedResource
}

func NewInMemoryCache() Cache {
	localCache := &localCache{
		cache:          make(map[string]resource.Resource),
		managedObjects: make(map[string]resource.ManagedResource),
	}

	return localCache
}

// Has implements Cache
func (l *localCache) Has(key string) bool {
	_, ok := l.cache[key]
	return ok
}

func (l *localCache) Get(key string) resource.Resource {
	res, ok := l.cache[key]
	if !ok {
		return resource.Resource{}
	}

	return res
}

func (l *localCache) GetManagedObject(key string) resource.ManagedResource {
	res, ok := l.managedObjects[key]
	if !ok {
		return resource.ManagedResource{}
	}

	return res
}

func (l *localCache) Set(key string, resource resource.Resource) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cache[key] = resource
}

func (l *localCache) SetManagedObject(key string, resource resource.ManagedResource) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.managedObjects[key] = resource
}

func (l *localCache) List() []string {
	keys := []string{}

	for key := range l.cache {
		keys = append(keys, key)
	}

	return keys
}

func (l *localCache) ListManagedObjects() []string {
	keys := []string{}

	for key := range l.managedObjects {
		keys = append(keys, key)
	}

	return keys
}

func (l *localCache) Delete(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.cache, key)
}
