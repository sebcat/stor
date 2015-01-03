package stor

import (
	"bytes"
	"testing"
	"time"
)

var testData = []byte{1, 2, 3, 251}
var testData2 = []byte{1, 5, 7, 1, 1, 255, 2}

func TestPutGet(t *testing.T) {
	s := Store{Path: "foo"}
	defer s.Remove() // test dual removal

	if err := s.Put("foo", testData); err != nil {
		t.Fatal(err)
	}

	// we will *probably* hit the transfer cache here
	// which we want
	if retrieved, err := s.Get("foo"); err != nil {
		t.Fatal(err)
	} else if bytes.Compare(testData, retrieved) != 0 {
		t.Fatal(err)
	}

	err := s.Remove()
	if err != nil {
		t.Fatal(err)
	}
}

func TestInventory(t *testing.T) {
	s := Store{Path: "foo", Inventory: NewDefaultInventory()}
	defer s.Remove()

	if el, err := s.Get("foo"); el != nil || err != ErrDoesNotExist {
		t.Fatal("expected ErrDoesNotExist, got", err)
	}

	if err := s.Put("foo", testData); err != nil {
		t.Fatal("unexpected error", err)
	}

	// elements are only added to inventory after succesful write
	s.Sync()
	if err := s.Put("foo", testData); err != ErrAlreadyExist {
		t.Fatal("expected ErrAlreadyExist, got", err)
	}
}

func testCacheInsertion(t *testing.T, c Cache) {
	s := Store{Path: "foo", Cache: c}
	defer s.Remove()

	s.Put("foo", testData)

	// we sync in order to avoid hitting the
	// transfer cache instead of the read-back cache
	s.Sync()

	// We read the element twice because on a retrieval cache
	// the element is inserted in the
	// cache the first time it's retrieved
	s.Get("foo")
	val, err := s.Get("foo")
	if err != nil || bytes.Compare(val, testData) != 0 {
		t.Fatal("expected", testData, "was", val)
	}
}

func TestCacheMostRecentInsertion(t *testing.T) {
	c := NewCacheMostRecent(1)
	testCacheInsertion(t, c)
}

func TestCacheAllInsertion(t *testing.T) {
	c := NewCacheAll()
	testCacheInsertion(t, c)
}

func testCacheOverwrite(t *testing.T, c Cache) {
	s := Store{Path: "foo", Cache: c}
	defer s.Remove()

	s.Put("foo", testData)
	s.Sync()
	s.Get("foo")
	data, err := s.Get("foo")
	if err != nil || bytes.Compare(data, testData) != 0 {
		t.Fatal("expected", testData, "was", data)
	}

	s.Put("foo", testData2)
	s.Sync()
	s.Get("foo")
	data, err = s.Get("foo")
	if err != nil || bytes.Compare(data, testData2) != 0 {
		t.Fatal("expected", testData2, "was", data)
	}
}

func TestCacheMostRecentOverwrite(t *testing.T) {
	c := NewCacheMostRecent(1)
	testCacheOverwrite(t, c)
}

func TestCacheAllOverwrite(t *testing.T) {
	c := NewCacheAll()
	testCacheOverwrite(t, c)
}

func TestCacheMostRecent(t *testing.T) {
	// logs retrieval times
	// go test -v -run 'CacheMostRecent$'
	c := NewCacheMostRecent(1)
	s := Store{Path: "foo", Cache: c}
	defer s.Remove()

	s.Put("foo", testData)
	s.Put("bar", testData2)
	s.Sync()

	var get = func(index int, hash string, expected []byte) {
		start := time.Now()
		data, err := s.Get(hash)
		t.Log("get", index, hash, time.Since(start))
		if err != nil {
			t.Fatal(err)
		}

		if bytes.Compare(data, expected) != 0 {
			t.Fatal("expected", expected, "was", data)
		}
	}

	for i := 0; i < 10; i++ {
		get(i, "foo", testData)  // expected miss
		get(i, "foo", testData)  // expected hit
		get(i, "foo", testData)  // expected hit
		get(i, "bar", testData2) // expected miss
	}
}

func TestCacheMostRecentExpiry(t *testing.T) {
	c := NewCacheMostRecent(1)
	s := Store{Path: "foo", Cache: c}
	defer s.Remove()

	s.Put("foo", testData)
	s.Sync()
	s.Get("foo")
	data, err := s.Get("foo")
	if err != nil || bytes.Compare(data, testData) != 0 {
		t.Fatal("expected", testData, "was", data)
	}

	s.Put("bar", testData2)
	s.Sync()
	s.Get("bar")
	data, err = s.Get("bar")
	if err != nil || bytes.Compare(data, testData2) != 0 {
		t.Fatal("expected", testData2, "was", data)
	}

	data, err = s.Get("foo")
	if err != nil || bytes.Compare(data, testData) != 0 {
		t.Fatal("expected", testData, "was", data)
	}
}

func benchGet(b *testing.B, s *Store, key string, data []byte) {
	if err := s.Put(key, data); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if retrieved, err := s.Get(key); err != nil {
			b.Fatal(err)
		} else if bytes.Compare(data, retrieved) != 0 {
			b.Fatal(err)
		}
	}

	b.StopTimer()
}

func BenchmarkGet(b *testing.B) {
	s := Store{Path: "foo"}
	defer s.Remove()
	benchGet(b, &s, "foo", testData)
}

func BenchmarkGetCacheAll(b *testing.B) {
	c := NewCacheAll()
	s := Store{Path: "foo", Cache: c}
	defer s.Remove()
	benchGet(b, &s, "foo", testData)
}

func BenchmarkGetCacheMostRecent100Hit(b *testing.B) {
	c := NewCacheMostRecent(1)
	s := Store{Path: "foo", Cache: c}
	defer s.Remove()
	benchGet(b, &s, "foo", testData)
}
