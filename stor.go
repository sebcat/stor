package stor

/*
 stor - a file system k/v-store
   keys: value hashes
   values: byte slices
*/

import (
	"container/list"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	ErrInvalidHash   = errors.New("invalid lookup hash")
	ErrLimiterDenied = errors.New("Store limiter denied insertion")
	ErrDoesNotExist  = errors.New("does not exist in store")
	ErrAlreadyExist  = errors.New("element already exist in store")
)

type Cache interface {
	// gets called when an element is retrieved from a store
	// nil is returned if the element does not exist in cache
	Get(hash string) (data []byte)
}

type RetrievalCache interface {
	Cache
	// gets called when an element is retrieved from a store
	SeeGet(hash string, data []byte)
}

type InsertionCache interface {
	Cache
	// gets called when an element is put into a store
	SeePut(hash string, data []byte)
}

type CacheAll struct {
	m     map[string][]byte
	mlock sync.RWMutex
}

func NewCacheAll() *CacheAll {
	return &CacheAll{m: make(map[string][]byte)}
}

func (c *CacheAll) SeePut(hash string, data []byte) {
	if data == nil {
		return
	}

	c.mlock.Lock()
	c.m[hash] = data
	c.mlock.Unlock()
}

func (c *CacheAll) Get(hash string) (data []byte) {
	c.mlock.RLock()
	el, exists := c.m[hash]
	c.mlock.RUnlock()
	if exists {
		return el
	} else {
		return nil
	}
}

type cacheElem struct {
	hash string
	data []byte
}

// Least Recently Used (LRU) eviction policy on retrieved elements
type CacheMostRecent struct {
	m        map[string]*list.Element
	l        *list.List
	capacity int
	mutex    sync.RWMutex
}

func NewCacheMostRecent(capacity int) *CacheMostRecent {
	if capacity <= 0 {
		return nil
	}

	return &CacheMostRecent{
		m:        make(map[string]*list.Element, capacity),
		l:        list.New(),
		capacity: capacity,
	}
}

// See a store retrieval. This should only occur if the element
// is not already in the cache
func (c *CacheMostRecent) SeeGet(hash string, data []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.l.Len() < c.capacity {
		listEl := c.l.PushFront(&cacheElem{hash: hash, data: data})
		c.m[hash] = listEl
	} else {
		tail := c.l.Back()
		tailVal := tail.Value.(*cacheElem)
		tail.Value = &cacheElem{hash: hash, data: data}
		c.l.MoveToFront(tail)
		delete(c.m, tailVal.hash)
		c.m[hash] = tail
	}
}

func (c *CacheMostRecent) SeePut(hash string, data []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if listEl, exists := c.m[hash]; exists {
		// an existing element in the cache is overwritten
		// but not moved to the front
		el := listEl.Value.(*cacheElem)
		el.hash = hash
		el.data = data
	}
}

// get data from the cache, if any
func (c *CacheMostRecent) Get(hash string) (data []byte) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if cacheEl, exists := c.m[hash]; exists {
		c.l.MoveToFront(cacheEl)
		return cacheEl.Value.(*cacheElem).data
	}

	return nil
}

// A Limiter can deny an element to be inserted into the Store
type Limiter interface {
	// returns true if the element is not to be inserted
	Deny(hash string, data []byte) bool
}

// an Inventory keeps track of inserted elements.
// On retrieval, if the inventory don't know the hash, a retrieval should be denied
// On insertion, if the inventory knows the hash, the insertion should be denied
type Inventory interface {
	See(hash string)
	Has(hash string) bool
}

type DefaultInventory struct {
	m     map[string]struct{}
	mlock sync.RWMutex
}

func NewDefaultInventory() *DefaultInventory {
	return &DefaultInventory{m: make(map[string]struct{})}
}

func (i *DefaultInventory) See(hash string) {
	var x struct{}
	i.mlock.Lock()
	i.m[hash] = x
	i.mlock.Unlock()
}

func (i *DefaultInventory) Has(hash string) bool {
	i.mlock.RLock()
	_, exists := i.m[hash]
	i.mlock.RUnlock()
	return exists
}

type Store struct {
	Limiter   Limiter
	Cache     Cache
	Inventory Inventory
	// Absolute or relative path in file system to store.
	// If not set,the current directory is used.
	// Be aware that Remove removes this path
	Path string

	// writeErr is set on asyncronous write error
	writeErr     error
	activeWrites sync.WaitGroup

	// While an element is being written to disk, it may be retrieved
	// by Get, in which case it will be retrieved from this
	// hopefully small cache.
	inTransfer     map[string][]byte
	inTransferLock sync.RWMutex
}

func (s *Store) hashDir(hash string) string {
	var subdir string
	if len(hash) < 2 {
		subdir = hash
	} else {
		subdir = hash[:2]
	}

	return filepath.Join(s.Path, subdir)
}

func (s *Store) put(hash string, data []byte) error {
	dir := s.hashDir(hash)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	fn := filepath.Join(dir, hash)
	fh, err := os.Create(fn)
	if err != nil {
		return err
	}

	defer fh.Close()
	_, err = fh.Write(data)
	return err
}

// Put an Element into a store. hash must be unique
// and the element must not already exist in the store
func (s *Store) Put(hash string, data []byte) error {
	if len(hash) == 0 || strings.Contains(hash, string(filepath.Separator)) {
		return ErrInvalidHash
	}

	if s.writeErr != nil {
		return s.writeErr
	}

	if s.Limiter != nil && s.Limiter.Deny(hash, data) {
		return ErrLimiterDenied
	}

	if s.Inventory != nil && s.Inventory.Has(hash) {
		return ErrAlreadyExist
	}

	if s.inTransfer == nil {
		s.inTransfer = make(map[string][]byte)
	}

	s.inTransferLock.Lock()
	s.inTransfer[hash] = data
	s.inTransferLock.Unlock()

	if s.Cache != nil {
		if putCache, ok := s.Cache.(InsertionCache); ok {
			putCache.SeePut(hash, data)
		}
	}

	s.activeWrites.Add(1)
	go func() {
		defer s.activeWrites.Done()

		defer func() {
			s.inTransferLock.Lock()
			delete(s.inTransfer, hash)
			s.inTransferLock.Unlock()
		}()

		if err := s.put(hash, data); err != nil {
			s.writeErr = err
		} else {
			if s.Inventory != nil {
				s.Inventory.See(hash)
			}
		}
	}()

	return nil
}

func (s *Store) get(hash string) ([]byte, error) {
	fn := filepath.Join(s.hashDir(hash), hash)
	fh, err := os.Open(fn)
	if err != nil {
		return nil, ErrDoesNotExist
	}

	defer fh.Close()
	return ioutil.ReadAll(fh)
}

// Get an element from a store by it's unique hashentifier
func (s *Store) Get(hash string) ([]byte, error) {
	if len(hash) == 0 || strings.Contains(hash, string(filepath.Separator)) {
		return nil, ErrInvalidHash
	}

	// main cache lookup before transfer cache lookup and inventory check
	if s.Cache != nil {
		if b := s.Cache.Get(hash); b != nil {
			return b, nil
		}
	}

	if s.inTransfer != nil {
		s.inTransferLock.RLock()
		b, ok := s.inTransfer[hash]
		s.inTransferLock.RUnlock()
		if ok {
			return b, nil
		}
	}

	if s.Inventory != nil {
		if !s.Inventory.Has(hash) {
			return nil, ErrDoesNotExist
		}
	}

	data, err := s.get(hash)
	if err == nil && s.Cache != nil {
		if getCache, ok := s.Cache.(RetrievalCache); ok {
			getCache.SeeGet(hash, data)
		}
	}

	return data, err
}

// Wait for all writes to be completed
func (s *Store) Sync() {
	s.activeWrites.Wait()
}

// Remove the store from the fle system
func (s *Store) Remove() error {
	s.Sync()
	return os.RemoveAll(s.Path)
}
