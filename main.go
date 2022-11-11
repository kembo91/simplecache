package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"
)

type Cache[K comparable, D any] interface {
	Get(key K) (D, bool)
	Set(key K, value D)
	Delete(key K)
	Close()
}

type dataWrapper[K comparable, D any] struct {
	key      K
	value    D
	deleteAt time.Time
}

type readType[K comparable, D any] struct {
	returnChan chan *D
	key        K
}

type writeType[K comparable, D any] struct {
	key   K
	value D
}

type cache[K comparable, D any] struct {
	ctx         context.Context
	data        map[K]*D
	orderedData []*dataWrapper[K, D]
	writeChan   chan writeType[K, D]
	readChan    chan readType[K, D]
	deleteChan  chan K
	ttl         time.Duration
	cancel      context.CancelFunc
}

func NewCache[K comparable, D any](ttl time.Duration) Cache[K, D] {
	ctx, cancel := context.WithCancel(context.Background())
	rv := &cache[K, D]{
		data:        make(map[K]*D),
		writeChan:   make(chan writeType[K, D]),
		ttl:         ttl,
		readChan:    make(chan readType[K, D]),
		deleteChan:  make(chan K),
		ctx:         ctx,
		cancel:      cancel,
		orderedData: make([]*dataWrapper[K, D], 0),
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
				dr := &dataWrapper[K, D]{
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

func (c *cache[K, D]) Get(key K) (D, bool) {
	retc := make(chan *D)
	defer close(retc)
	rc := readType[K, D]{returnChan: retc, key: key}
	c.readChan <- rc
	got := <-rc.returnChan
	if got == nil {
		dd := new(D)
		return *dd, false
	}
	return *got, true
}

func (c *cache[K, D]) Set(key K, value D) {
	c.writeChan <- writeType[K, D]{key: key, value: value}
}

func (c *cache[K, D]) Delete(key K) {
	c.deleteChan <- key
}

func (c *cache[K, D]) Close() {
	c.cancel()
}

func main() {
	cache := NewCache[string, string](time.Second * 3)
	cache.Set("hello", "world")
	got, ok := cache.Get("hello")
	if ok {
		fmt.Println(got)
	} else {
		fmt.Println("something went wrong")
	}
	wg := sync.WaitGroup{}
	for i := 0; i < 1000; i++ {
		go func(i int) {
			defer wg.Done()
			wg.Add(1)
			cache.Set(strconv.Itoa(i), "yoyo")
		}(i)
	}
	wg.Wait()
	for i := 0; i < 1000; i++ {
		go func(i int) {
			defer wg.Done()
			wg.Add(1)
			res, ok := cache.Get(strconv.Itoa(i))
			if !ok {
				fmt.Println("not ok")
			}
			if res != "yoyo" {
				fmt.Println("not yoyo")
			}
		}(i)
	}
	wg.Wait()
	cache.Close()
}
