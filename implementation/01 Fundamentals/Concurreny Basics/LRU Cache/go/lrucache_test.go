// lrucache_test.go
package lrucache

import (
    "sync"
    "testing"
)

func TestGet_ReturnsZeroValue_WhenKeyNotPresent(t *testing.T) {
    cache, _ := New[string, int](3)
    val, ok := cache.Get("missing")
    if ok || val != 0 {
        t.Errorf("expected (0, false), got (%d, %v)", val, ok)
    }
}

func TestPutAndGet_BasicOperation(t *testing.T) {
    cache, _ := New[string, int](3)
    cache.Put("a", 1)
    val, ok := cache.Get("a")
    if !ok || val != 1 {
        t.Errorf("expected (1, true), got (%d, %v)", val, ok)
    }
}

func TestPut_UpdatesExistingKey(t *testing.T) {
    cache, _ := New[string, int](3)
    cache.Put("a", 1)
    cache.Put("a", 99)
    val, _ := cache.Get("a")
    if val != 99 {
        t.Errorf("expected 99, got %d", val)
    }
    if cache.Size() != 1 {
        t.Errorf("expected size 1, got %d", cache.Size())
    }
}

func TestEviction_RemovesLeastRecentlyUsed(t *testing.T) {
    cache, _ := New[string, int](3)
    cache.Put("a", 1)
    cache.Put("b", 2)
    cache.Put("c", 3)
    cache.Put("d", 4) // should evict "a"

    if _, ok := cache.Get("a"); ok {
        t.Error("expected 'a' to be evicted")
    }
}

func TestGet_RefreshesRecency(t *testing.T) {
    cache, _ := New[string, int](3)
    cache.Put("a", 1)
    cache.Put("b", 2)
    cache.Put("c", 3)
    cache.Get("a")      // "a" becomes MRU — "b" becomes LRU
    cache.Put("d", 4)   // should evict "b"

    if _, ok := cache.Get("b"); ok {
        t.Error("expected 'b' to be evicted")
    }
    if _, ok := cache.Get("a"); !ok {
        t.Error("expected 'a' to still be present")
    }
}

func TestNew_ReturnsError_OnInvalidCapacity(t *testing.T) {
    _, err := New[string, int](0)
    if err == nil {
        t.Error("expected error for capacity 0")
    }
    _, err = New[string, int](-1)
    if err == nil {
        t.Error("expected error for negative capacity")
    }
}

func TestConcurrentPuts_DoNotCorruptCache(t *testing.T) {
    cache, _ := New[int, int](100)
    var wg sync.WaitGroup
    goroutines := 20
    opsEach := 500

    for g := 0; g < goroutines; g++ {
        wg.Add(1)
        go func(gID int) {
            defer wg.Done()
            for i := 0; i < opsEach; i++ {
                cache.Put(gID*opsEach+i, i)
            }
        }(g)
    }

    wg.Wait()

    if cache.Size() > 100 {
        t.Errorf("cache exceeded capacity: size=%d", cache.Size())
    }
}