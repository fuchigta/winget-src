package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSHA256Fetcher_CacheHit(t *testing.T) {
	var callCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Write([]byte("hello"))
	}))
	defer ts.Close()

	fetcher := NewSHA256Fetcher(ts.Client(), time.Minute)

	hash1, err := fetcher.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("first Fetch: %v", err)
	}

	hash2, err := fetcher.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("expected same hash, got %s and %s", hash1, hash2)
	}
	if callCount.Load() != 1 {
		t.Errorf("expected 1 HTTP call, got %d", callCount.Load())
	}
}

func TestSHA256Fetcher_InflightDeduplication(t *testing.T) {
	var callCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte("hello"))
	}))
	defer ts.Close()

	fetcher := NewSHA256Fetcher(ts.Client(), time.Minute)

	const goroutines = 5
	results := make([]string, goroutines)
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			results[i], errs[i] = fetcher.Fetch(context.Background(), ts.URL)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
	for i := 1; i < goroutines; i++ {
		if results[i] != results[0] {
			t.Errorf("goroutine %d returned %s, want %s", i, results[i], results[0])
		}
	}
	if callCount.Load() != 1 {
		t.Errorf("expected 1 HTTP call due to deduplication, got %d", callCount.Load())
	}
}

// TestSHA256Fetcher_ContextCancelledButComputationContinues は呼び出し元のctxがキャンセルされても
// バックグラウンドでダウンロードが継続し、次のリクエストがキャッシュから即座に返ることを確認する。
func TestSHA256Fetcher_ContextCancelledButComputationContinues(t *testing.T) {
	downloadDone := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ダウンロードが完了したことを通知（コンテキストに依存しないことを確認するため）
		defer close(downloadDone)
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("hello"))
	}))
	defer ts.Close()

	fetcher := NewSHA256Fetcher(ts.Client(), time.Minute)

	// 短いタイムアウトで1回目のリクエスト → コンテキストがキャンセルされる
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := fetcher.Fetch(ctx, ts.URL)
	if err == nil {
		t.Fatal("expected context deadline exceeded error")
	}

	// バックグラウンドダウンロードが完了するまで待つ
	select {
	case <-downloadDone:
	case <-time.After(2 * time.Second):
		t.Fatal("background download did not complete in time")
	}

	// 2回目のリクエストはキャッシュから即座に返るはず
	hash, err := fetcher.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("second Fetch after cache should succeed: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("expected 64-char SHA256, got %d chars: %s", len(hash), hash)
	}
}

func TestSHA256Fetcher_ErrorNotCached(t *testing.T) {
	var callCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	fetcher := NewSHA256Fetcher(ts.Client(), time.Minute)

	_, err := fetcher.Fetch(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	// エラーはキャッシュされないため2回目も試みる
	_, err = fetcher.Fetch(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected error for 500 response on second call")
	}

	if callCount.Load() != 2 {
		t.Errorf("expected 2 HTTP calls (errors not cached), got %d", callCount.Load())
	}
}

// TestSHA256Fetcher_Seed は Seed で投入した URL→SHA256 マッピングがキャッシュヒットすることを確認する。
func TestSHA256Fetcher_Seed(t *testing.T) {
	var callCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Write([]byte("hello"))
	}))
	defer ts.Close()

	const expectedHash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	fetcher := NewSHA256Fetcher(ts.Client(), time.Minute)
	fetcher.Seed(map[string]string{ts.URL: expectedHash})

	// Seed 済みなので HTTP 呼び出しは発生しない
	hash, err := fetcher.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Fetch after Seed: %v", err)
	}
	if hash != expectedHash {
		t.Errorf("expected hash %s, got %s", expectedHash, hash)
	}
	if callCount.Load() != 0 {
		t.Errorf("expected 0 HTTP calls (seed cache hit), got %d", callCount.Load())
	}
}
