package cache

import (
	"bufio"
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
		for {
			f, err := os.Create(fmt.Sprintf("data/cache-%d", time.Now().Unix()))
			if err != nil {
				panic(err)
			}

			w := bufio.NewWriter(f)
			for key, value := range localCache.cache {
				if value.Object != nil {
					_, err := w.WriteString(fmt.Sprintf("%s\n", key))
					if err != nil {
						panic(err)
					}
				}

			}

			w.Flush()

			fmt.Printf("persist %d cache items\n", len(localCache.cache))

			<-time.After(20 * time.Second)
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

func (l *localCache) Scan(filter func(res resource.Resource) bool) map[string]resource.Resource {
	keys := map[string]resource.Resource{}

	for key, value := range l.cache {
		if filter(value) {
			keys[key] = value
		}
	}

	return keys
}

func (l *localCache) Delete(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.cache, key)
}
