package option

import "goCache/goCache/cache"

type CacheOption struct {
	Strategy cache.Cache
}

type CacheOptionFunc func(option *CacheOption)

func WithCacheOptionsStrategy(Strategy string, maxBytes int64, evictedFunc cache.OnEvictedFunc) func(option *CacheOption) {
	return func(option *CacheOption) {
		switch Strategy {
		case "lru":
			option.Strategy = cache.NewLRU(maxBytes, evictedFunc)
		default:
			option.Strategy = cache.NewLRU(maxBytes, evictedFunc)
		}
	}
}

func DefaultCacheOption() CacheOption {
	return CacheOption{Strategy: cache.NewLRU(0, nil)}
}
