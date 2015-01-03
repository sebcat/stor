# stor
--
    import "github.com/sebcat/stor"


## Usage

```go
var (
	ErrInvalidHash   = errors.New("invalid lookup hash")
	ErrLimiterDenied = errors.New("Store limiter denied insertion")
	ErrDoesNotExist  = errors.New("does not exist in store")
	ErrAlreadyExist  = errors.New("element already exist in store")
)
```

#### type Cache

```go
type Cache interface {
	// gets called when an element is retrieved from a store
	// nil is returned if the element does not exist in cache
	Get(hash string) (data []byte)
}
```


#### type CacheAll

```go
type CacheAll struct {
}
```


#### func  NewCacheAll

```go
func NewCacheAll() *CacheAll
```

#### func (*CacheAll) Get

```go
func (c *CacheAll) Get(hash string) (data []byte)
```

#### func (*CacheAll) SeePut

```go
func (c *CacheAll) SeePut(hash string, data []byte)
```

#### type CacheMostRecent

```go
type CacheMostRecent struct {
}
```

Least Recently Used (LRU) eviction policy on retrieved elements

#### func  NewCacheMostRecent

```go
func NewCacheMostRecent(capacity int) *CacheMostRecent
```

#### func (*CacheMostRecent) Get

```go
func (c *CacheMostRecent) Get(hash string) (data []byte)
```
get data from the cache, if any

#### func (*CacheMostRecent) SeeGet

```go
func (c *CacheMostRecent) SeeGet(hash string, data []byte)
```
See a store retrieval. This should only occur if the element is not already in
the cache

#### func (*CacheMostRecent) SeePut

```go
func (c *CacheMostRecent) SeePut(hash string, data []byte)
```

#### type DefaultInventory

```go
type DefaultInventory struct {
}
```


#### func  NewDefaultInventory

```go
func NewDefaultInventory() *DefaultInventory
```

#### func (*DefaultInventory) Has

```go
func (i *DefaultInventory) Has(hash string) bool
```

#### func (*DefaultInventory) See

```go
func (i *DefaultInventory) See(hash string)
```

#### type InsertionCache

```go
type InsertionCache interface {
	Cache
	// gets called when an element is put into a store
	SeePut(hash string, data []byte)
}
```


#### type Inventory

```go
type Inventory interface {
	See(hash string)
	Has(hash string) bool
}
```

an Inventory keeps track of inserted elements. On retrieval, if the inventory
don't know the hash, a retrieval should be denied On insertion, if the inventory
knows the hash, the insertion should be denied

#### type Limiter

```go
type Limiter interface {
	// returns true if the element is not to be inserted
	Deny(hash string, data []byte) bool
}
```

A Limiter can deny an element to be inserted into the Store

#### type RetrievalCache

```go
type RetrievalCache interface {
	Cache
	// gets called when an element is retrieved from a store
	SeeGet(hash string, data []byte)
}
```


#### type Store

```go
type Store struct {
	Limiter   Limiter
	Cache     Cache
	Inventory Inventory
	// Absolute or relative path in file system to store.
	// If not set,the current directory is used.
	// Be aware that Remove removes this path
	Path string
}
```


#### func (*Store) Get

```go
func (s *Store) Get(hash string) ([]byte, error)
```
Get an element from a store by it's unique hashentifier

#### func (*Store) Put

```go
func (s *Store) Put(hash string, data []byte) error
```
Put an Element into a store. hash must be unique and the element must not
already exist in the store

#### func (*Store) Remove

```go
func (s *Store) Remove() error
```
Remove the store from the fle system

#### func (*Store) Sync

```go
func (s *Store) Sync()
```
Wait for all writes to be completed


## Example

with proper GOPATH setup:

```
$ go get github.com/sebcat/stor
$ go test -v -bench . github.com/sebcat/stor
```
