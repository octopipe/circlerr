package cache

import (
	"sync"

	"github.com/octopipe/circlerr/pkg/twice/resource"
)

type localCache struct {
	mu sync.RWMutex

	cache map[string]resource.Resource
}

func NewLocalCache() Cache {
	return &localCache{
		cache: make(map[string]resource.Resource),
	}
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

func (l *localCache) Set(key string, resource resource.Resource) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cache[key] = resource
}

func (l *localCache) List(filter func(res resource.Resource) bool) []string {
	list := []string{}

	for key := range l.cache {
		if filter(l.cache[key]) {
			list = append(list, key)
		}
	}

	return list
}

func (l *localCache) Delete(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.cache, key)
}
