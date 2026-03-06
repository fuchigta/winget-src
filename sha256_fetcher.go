package main

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type sha256Job struct {
	done   chan struct{}
	result string
	err    error
}

// SHA256Fetcher はURLのSHA256ハッシュをインメモリキャッシュ・重複排除しながら計算する。
// ダウンロードはcontext.Background()で行うため、呼び出し元のコンテキストがキャンセルされても
// バックグラウンドで計算が続き、次のリクエスト時にキャッシュから即座に返せる。
type SHA256Fetcher struct {
	mu        sync.Mutex
	cache     map[string]string
	inflight  map[string]*sha256Job
	client    *http.Client
	bgTimeout time.Duration
}

// NewSHA256Fetcher は新しい SHA256Fetcher を返す。
// client はダウンロード用HTTPクライアント（タイムアウトなし推奨）。
// bgTimeout はバックグラウンド計算のタイムアウト。
func NewSHA256Fetcher(client *http.Client, bgTimeout time.Duration) *SHA256Fetcher {
	return &SHA256Fetcher{
		cache:     make(map[string]string),
		inflight:  make(map[string]*sha256Job),
		client:    client,
		bgTimeout: bgTimeout,
	}
}

// Seed は URL→SHA256 マッピングをインメモリキャッシュに一括投入する。
// version_cache.json からロードした SHA256 を事前投入するために使用する。
func (f *SHA256Fetcher) Seed(urlToSHA256 map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for url, hash := range urlToSHA256 {
		f.cache[url] = hash
	}
}

// Fetch は指定URLのSHA256ハッシュ（hex）を返す。
//   - キャッシュヒット: 即座に返す
//   - 進行中（inflight）: ctx でウェイト（キャンセルされても計算は続く）
//   - ミス: context.Background() でバックグラウンドゴルーチンを起動し、ctx でウェイト
func (f *SHA256Fetcher) Fetch(ctx context.Context, url string) (string, error) {
	f.mu.Lock()
	if cached, ok := f.cache[url]; ok {
		f.mu.Unlock()
		return cached, nil
	}
	if job, ok := f.inflight[url]; ok {
		f.mu.Unlock()
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-job.done:
			return job.result, job.err
		}
	}

	job := &sha256Job{done: make(chan struct{})}
	f.inflight[url] = job
	f.mu.Unlock()

	go func() {
		defer close(job.done)

		bgCtx, cancel := context.WithTimeout(context.Background(), f.bgTimeout)
		defer cancel()

		result, err := computeSHA256FromURL(bgCtx, f.client, url)
		job.result = result
		job.err = err

		f.mu.Lock()
		delete(f.inflight, url)
		if err == nil {
			f.cache[url] = result
		}
		f.mu.Unlock()

		if err != nil {
			slog.Warn("sha256 background fetch failed", "url", url, "error", err)
		}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-job.done:
		return job.result, job.err
	}
}
