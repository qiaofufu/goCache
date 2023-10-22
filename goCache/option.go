package goCache

import "goCache/goCache/cache"

type CacheOption struct {
	mainCache cache.Cache
	hotCache  cache.Cache
}

type CacheOptionFunc func(option *CacheOption)

func WithCacheOptionsStrategy(Strategy string, maxBytes int64, evictedFunc cache.OnEvictedFunc) func(option *CacheOption) {
	return func(option *CacheOption) {
		switch Strategy {
		case "lru":
			option.mainCache = cache.NewLRU(maxBytes, evictedFunc)
			option.hotCache = cache.NewLRU(maxBytes/8, evictedFunc)
		default:
			option.mainCache = cache.NewLRU(maxBytes, evictedFunc)
			option.hotCache = cache.NewLRU(maxBytes/8, evictedFunc)
		}
	}
}

func DefaultCacheOption() CacheOption {
	return CacheOption{mainCache: cache.NewLRU(0, nil)}
}
