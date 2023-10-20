package option

import "goCache/server/storage"

type CacheOption struct {
	Strategy storage.Storage
}

type CacheOptionFunc func(option *CacheOption)

func WithCacheOptionsStrategy(Strategy string, maxBytes int64, evictedFunc storage.OnEvictedFunc) func(option *CacheOption) {
	return func(option *CacheOption) {
		switch Strategy {
		case "lru":
			option.Strategy = storage.NewLRU(maxBytes, evictedFunc)
		default:
			option.Strategy = storage.NewLRU(maxBytes, evictedFunc)
		}
	}
}

func DefaultCacheOption() CacheOption {
	return CacheOption{Strategy: storage.NewLRU(0, nil)}
}
