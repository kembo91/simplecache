package simplecache

import (
	"context"
	"time"
)

type Cache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V)
	Delete(key K)
	Close()
}

type dataWrapper[K comparable, V any] struct {
	key      K
	value    V
	deleteAt time.Time
}

type readType[K comparable, V any] struct {
	returnChan chan *V
	key        K
}

type writeType[K comparable, V any] struct {
	key   K
	value V
}

type cache[K comparable, V any] struct {
	ctx         context.Context
	data        map[K]*V
	orderedData []*dataWrapper[K, V]
	writeChan   chan writeType[K, V]
	readChan    chan readType[K, V]
	deleteChan  chan K
	ttl         time.Duration
	cancel      context.CancelFunc
}

func NewCache[K comparable, V any]() Cache[K, V] {
	return newCache[K, V]()
}

func NewTTLCache[K comparable, V any](ttl time.Duration) Cache[K, V] {
	rv := newCache[K, V]()
	rv.ttl = ttl
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				for ix, v := range rv.orderedData {
					if time.Now().After(v.deleteAt) {
						rv.deleteChan <- v.key
					} else {
						rv.orderedData = rv.orderedData[ix:]
						break
					}
				}
			case <-rv.ctx.Done():
				return
			}
		}
	}()
	return rv
}

func newCache[K comparable, V any]() *cache[K, V] {
	ctx, cancel := context.WithCancel(context.Background())
	rv := &cache[K, V]{
		data:        make(map[K]*V),
		writeChan:   make(chan writeType[K, V]),
		readChan:    make(chan readType[K, V]),
		deleteChan:  make(chan K),
		ctx:         ctx,
		cancel:      cancel,
		orderedData: make([]*dataWrapper[K, V], 0),
	}
	go func() {
		defer func() {
			close(rv.readChan)
			close(rv.writeChan)
			close(rv.deleteChan)
		}()
		for {
			select {
			case read := <-rv.readChan:
				rv, ok := rv.data[read.key]
				if !ok {
					read.returnChan <- nil
					continue
				}
				read.returnChan <- rv
			case write := <-rv.writeChan:
				dr := &dataWrapper[K, V]{
					deleteAt: time.Now().Add(rv.ttl),
					key:      write.key,
					value:    write.value,
				}
				rv.data[write.key] = &write.value
				rv.orderedData = append(rv.orderedData, dr)
			case del := <-rv.deleteChan:
				delete(rv.data, del)
			case <-rv.ctx.Done():
				return
			}
		}
	}()
	return rv
}

func (c *cache[K, V]) Get(key K) (V, bool) {
	retc := make(chan *V)
	defer close(retc)
	rc := readType[K, V]{returnChan: retc, key: key}
	c.readChan <- rc
	got := <-rc.returnChan
	if got == nil {
		dd := new(V)
		return *dd, false
	}
	return *got, true
}

func (c *cache[K, V]) Set(key K, value V) {
	c.writeChan <- writeType[K, V]{key: key, value: value}
}

func (c *cache[K, V]) Delete(key K) {
	c.deleteChan <- key
}

func (c *cache[K, V]) Close() {
	c.cancel()
}
