package main

import (
	"sync"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	c := NewCache[string](1*time.Minute, 0)

	c.Set("key1", "value1")
	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got '%s'", val)
	}
}

func TestCache_GetMiss(t *testing.T) {
	c := NewCache[string](1*time.Minute, 0)

	_, ok := c.Get("nonexistent")
	if ok {
		t.Fatal("expected key to not be found")
	}
}

func TestCache_TTLExpiration(t *testing.T) {
	c := NewCache[string](50*time.Millisecond, 0)

	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found before expiration")
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got '%s'", val)
	}

	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("key1")
	if ok {
		t.Fatal("expected key1 to be expired")
	}
}

func TestCache_Overwrite(t *testing.T) {
	c := NewCache[string](1*time.Minute, 0)

	c.Set("key1", "value1")
	c.Set("key1", "value2")

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if val != "value2" {
		t.Errorf("expected 'value2', got '%s'", val)
	}
}

func TestCache_MultipleKeys(t *testing.T) {
	c := NewCache[int](1*time.Minute, 0)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	for _, tt := range []struct {
		key  string
		want int
	}{
		{"a", 1},
		{"b", 2},
		{"c", 3},
	} {
		val, ok := c.Get(tt.key)
		if !ok {
			t.Errorf("expected key '%s' to be found", tt.key)
		}
		if val != tt.want {
			t.Errorf("key '%s': expected %d, got %d", tt.key, tt.want, val)
		}
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache[int](1*time.Minute, 0)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.Set("key", n)
			c.Get("key")
		}(i)
	}
	wg.Wait()

	_, ok := c.Get("key")
	if !ok {
		t.Fatal("expected key to be found after concurrent writes")
	}
}
