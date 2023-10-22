package singleflight

import "sync"

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

type Flight struct {
	mu sync.Mutex
	m  map[string]*call
}

func (f *Flight) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	f.mu.Lock()
	if f.m == nil {
		f.m = make(map[string]*call)
	}
	if call, ok := f.m[key]; ok {
		f.mu.Unlock()
		call.wg.Wait()
		return call.val, call.err
	}
	c := new(call)
	c.wg.Add(1)
	f.m[key] = c
	f.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	f.mu.Lock()
	delete(f.m, key)
	f.mu.Unlock()

	return c.val, c.err
}
