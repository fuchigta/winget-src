package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type sha256Job struct {
	done   chan struct{}
	result string
	err    error
}

// SHA256Fetcher はURLのSHA256ハッシュをキャッシュ・重複排除しながら計算する。
// ダウンロードはcontext.Background()で行うため、呼び出し元のコンテキストがキャンセルされても
// バックグラウンドで計算が続き、次のリクエスト時にキャッシュから即座に返せる。
type SHA256Fetcher struct {
	mu        sync.Mutex
	cache     map[string]string
	inflight  map[string]*sha256Job
	client    *http.Client
	bgTimeout time.Duration
	cacheFile string // 空文字列の場合はディスクキャッシュ無効
}

// NewSHA256Fetcher は新しい SHA256Fetcher を返す。
// client はダウンロード用HTTPクライアント（タイムアウトなし推奨）。
// bgTimeout はバックグラウンド計算のタイムアウト。
// cacheFile はディスクキャッシュファイルのパス（空文字列の場合は無効）。
func NewSHA256Fetcher(client *http.Client, bgTimeout time.Duration, cacheFile string) *SHA256Fetcher {
	f := &SHA256Fetcher{
		cache:     make(map[string]string),
		inflight:  make(map[string]*sha256Job),
		client:    client,
		bgTimeout: bgTimeout,
		cacheFile: cacheFile,
	}
	if cacheFile != "" {
		if err := f.loadDiskCache(); err != nil {
			slog.Warn("failed to load sha256 disk cache", "file", cacheFile, "error", err)
		}
	}
	return f
}

func (f *SHA256Fetcher) loadDiskCache() error {
	data, err := os.ReadFile(f.cacheFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var loaded map[string]string
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for k, v := range loaded {
		f.cache[k] = v
	}
	return nil
}

func (f *SHA256Fetcher) saveDiskCache() error {
	f.mu.Lock()
	snapshot := maps.Clone(f.cache)
	f.mu.Unlock()

	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	dir := filepath.Dir(f.cacheFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "sha256cache-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, f.cacheFile)
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

		if err == nil && f.cacheFile != "" {
			if saveErr := f.saveDiskCache(); saveErr != nil {
				slog.Warn("failed to save sha256 disk cache", "file", f.cacheFile, "error", saveErr)
			}
		}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-job.done:
		return job.result, job.err
	}
}
