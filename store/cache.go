package store

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

// ErrCachedExp returned by cache when cached record have expired,
// and no longer living in cache (deleted)
var ErrCachedExp = errors.New("cache: Cached record have expired")

// Cache stores data so that future requests for that data can be served faster.
type Cache interface {
	// Load returns the value stored in the cache for a key, or nil if no value is present.
	// The ok result indicates whether value was found in the Cache.
	// The error reserved for moderate cache and returned if an error occurs, Otherwise nil.
	Load(key string, r *http.Request) (interface{}, bool, error)
	// Store sets the value for a key.
	// The error reserved for moderate cache and returned if an error occurs, Otherwise nil.
	Store(key string, value interface{}, r *http.Request) error
	// Delete deletes the value for a key.
	// The error reserved for moderate cache and returned if an error occurs, Otherwise nil.
	Delete(key string, r *http.Request) error
}

// NewDefaultCache return a simple Cache instance safe for concurrent usage,
// And spawning a garbage collector goroutine to collect expired record.
// The cache send record to garbage collector through a queue when it stored a new one.
// Once the garbage collector received the record it checks if record not expired to wait until expiration,
// Otherwise, wait for the next record.
// When the all expired record collected the garbage collector will be blocked,
// until new record stored to repeat the process.
func NewDefaultCache(ttl time.Duration) Cache {
	queue := &queue{
		notify: make(chan struct{}, 1),
		mu:     &sync.Mutex{},
	}

	cache := &defaultCache{
		queue: queue,
		ttl:   ttl,
		Map:   new(sync.Map),
	}

	go gc(queue, cache)

	return cache
}

type record struct {
	exp   time.Time
	key   string
	value interface{}
}

type defaultCache struct {
	*sync.Map
	queue *queue
	ttl   time.Duration
}

func (d *defaultCache) Load(key string, _ *http.Request) (interface{}, bool, error) {
	v, ok := d.Map.Load(key)

	if !ok {
		return nil, ok, nil
	}

	record := v.(*record)

	if time.Now().UTC().After(record.exp) {
		d.Map.Delete(key)
		return nil, ok, ErrCachedExp
	}

	return record.value, ok, nil
}

func (d *defaultCache) Store(key string, value interface{}, _ *http.Request) error {
	exp := time.Now().UTC().Add(d.ttl)
	record := &record{
		key:   key,
		exp:   exp,
		value: value,
	}
	d.Map.Store(key, record)
	d.queue.push(record)
	return nil
}

func (d *defaultCache) Delete(key string, _ *http.Request) error {
	d.Map.Delete(key)
	return nil
}

type node struct {
	record *record
	next   *node
}

type queue struct {
	mu     *sync.Mutex
	head   *node
	tail   *node
	notify chan struct{}
}

func (q *queue) next() *record {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.head != nil {
		current := q.head
		q.head = current.next
		return current.record
	}
	return nil
}

func (q *queue) push(r *record) {
	q.mu.Lock()
	defer q.mu.Unlock()
	node := &node{
		record: r,
		next:   nil,
	}
	if q.head == nil {
		q.head = node
		q.tail = q.head
		q.notify <- struct{}{}
		return
	}
	q.tail.next = node
	q.tail = q.tail.next
}

func gc(queue *queue, cache *defaultCache) {
	for {
		record := queue.next()

		if record == nil {
			<-queue.notify
			continue
		}
		_, ok, _ := cache.Load(record.key, nil)

		// check if the record exist then wait until it expired
		if ok {
			d := record.exp.Sub(time.Now().UTC())
			<-time.After(d)
		}

		_ = cache.Delete(record.key, nil)
	}
}
