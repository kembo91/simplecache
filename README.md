# Simple generic in-memory cache
This is a generic key-value store, written in pure go

You can set any `comparable` type `K` as key. And `any` type `V` as value

Concurrent reads and writes are synchronized under the hood via channels.

There is also a TTL cache with +- 1 second accuracy
### How to use
```go
c := simplecache.NewCache[string, int]()
defer c.Close()
// For TTL use c := simplecache.NewTTLCache[K, D](ttl time.Duration)
c.Set("foo", 100)
got, ok := c.Get("foo")
if ok {
    fmt.Println(got) // 100
}
```
### How to install
```
go get -v -u github.com/kembo91/simplecache
```
