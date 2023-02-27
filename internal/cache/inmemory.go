package cache

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/octopipe/circlerr/internal/resource"
)

type localCache struct {
	mu sync.RWMutex

	cache map[string]resource.Resource
}

func NewInMemoryCache() Cache {
	localCache := &localCache{
		cache: make(map[string]resource.Resource),
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-ticker.C:
				f, err := os.Create(fmt.Sprintf("data/snapshot"))
				if err != nil {
					panic(err)
				}
				buf := bytes.Buffer{}
				for key, res := range localCache.cache {
					buf.WriteString(fmt.Sprintf("[%s]: %v\n", key, res))
				}
				f.Write(buf.Bytes())
			}
		}

	}()

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

func (l *localCache) Set(key string, resource resource.Resource) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cache[key] = resource
}

func (l *localCache) List() []string {
	keys := []string{}

	for key := range l.cache {
		fmt.Println(key)
		keys = append(keys, key)
	}

	return keys
}

func (l *localCache) Delete(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.cache, key)
}
