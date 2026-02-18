# TODO

## バグ・不具合

- [x] `github.go:28` — API URL構築で `entry.Publisher`/`entry.Name` を使用しているが、`entry.Id`（owner/repo形式）を使うべき
- [x] `github.go`, `gitlab.go` — `handleZipPortable` のループ内で `defer` を使用しており、関数終了までリソースが解放されない（リソースリーク）
- [x] `main.go:25` — typo: `pacakgeListPath` → `packageListPath`
- [x] `models.go:92` — typo: `ErrorReponseEntry` → `ErrorResponseEntry`

## テスト

- [x] ユニットテストが存在しない。Service層・Repository層のテスト追加が必要
- [x] プロバイダー層のモック化によるインテグレーションテストの追加

## コード品質

- [x] `github.go` / `gitlab.go` の `handleZipPortable` ロジックがほぼ同一。共通関数への抽出を検討
- [x] `repository.go` の `ById`/`ByName` で大文字小文字を無視した比較が未対応（`strings.EqualFold` や `strings.ToLower` の使用を検討）
- [x] `handler.go` で `json.NewEncoder().Encode()` の戻り値エラーを無視している
- [x] `http.DefaultClient` を使用しており、外部API呼び出しにタイムアウトが設定されていない

## 機能追加

- [x] レスポンスキャッシュの導入（毎リクエストで外部APIに問い合わせている）
- [x] `zip-portable` 以外のインストーラータイプ対応（msi, exe 等）
- [x] ヘルスチェックエンドポイント (`GET /health`) の追加

## バグ・不具合（追加）

- [x] `service.go:31,67` — typo: `conditons` → `conditions`, `maniests` → `manifests`
- [x] `repository.go:12` — typo: `QueryManifestConditon` → `QueryManifestCondition`（型名・インターフェース・テスト全箇所）
- [x] `provider_common.go:164-166` — `scanner.Err()` がループ内にあり、最後のエラーしか検出できない。ループ外に移動すべき
- [x] `handler.go:34` — `/information` エンドポイントで `service.Information()` のエラーを無視している

## コード品質（追加）

- [x] `repository.go:120-121` — `QueryPackageManifests` で identifier が見つからない場合に error を返しているが、handler側で500エラーになる。空の `PackageManifests` を返して204にすべき
- [x] `provider_common.go:158` — `fetchChecksums` でレスポンスボディの読み取りサイズに上限がない（DoS対策として `io.LimitReader` を使用すべき）

## テスト（追加）

- [x] `cache.go` のユニットテスト追加（TTL期限切れ、Get/Set、並行アクセス）
- [x] `provider_common.go` のユニットテスト追加（`detectArch`, `buildZipPortableVersions`, `buildInstallerVersions`）

## バグ・不具合（追加2）

- [x] `service.go:85` — バージョンフィルタリングの `break` が不要。意図が不明確なので削除する
- [x] `repository.go:114` — `QueryPackageManifests` の identifier 比較が `==` で大文字小文字を区別している。`strings.EqualFold` に統一すべき
- [x] `main.go:52-54` — `ListenAndServe` のエラーハンドリングが不正。`ErrServerClosed` 以外のエラー（ポート競合等）が無視される
- [x] `provider_common.go:27-35` — `detectArch` が `amd64`, `aarch64` に未対応

## コード品質（追加2）

- [x] `main.go:39-43` — HTTPサーバーに `ReadTimeout`, `WriteTimeout`, `IdleTimeout` が未設定
- [x] `github.go:43`, `gitlab.go:47` — APIエラーレスポンスの `io.ReadAll` にサイズ上限がない（`io.LimitReader` を適用すべき）
- [x] `types.go` — `InstallerType` の有効値（`zip-portable`, `msi`, `exe`）が定数定義されていない。switch 文にハードコードされている
- [x] `provider_common.go` — `buildZipPortableVersions` と `buildInstallerVersions` の共通ロジックを抽出して重複を削減する
- [x] `cache.go` — TTL期限切れエントリがマップに残り続けメモリリークする。定期削除の仕組みを追加する
- [x] `provider_common.go:12-14` — `httpClient` がグローバル変数でテスト時のモック化が困難。プロバイダー構造体のフィールドに注入する

## テスト（追加2）

- [x] `github.go`, `gitlab.go` のユニットテスト追加（`httptest` モックサーバーによるプロバイダー層テスト）

## 機能追加（追加）

- [x] 全層に `context.Context` を導入し、キャンセル・タイムアウト制御を可能にする
- [x] `gopkg.in/yaml.v2` → `v3` への移行

## セキュリティ

- [x] `handler.go:47` — `/manifestSearch` の `json.NewDecoder` にリクエストボディサイズ上限がない。大きなJSONペイロードでメモリ枯渇の可能性。`http.MaxBytesReader` で制限すべき
- [x] `provider_common.go:135`, `github.go:49`, `gitlab.go:48` — HTTPステータスコードの比較がマジックナンバー `200`。`http.StatusOK` 定数に統一すべき

## 設定の外部化

- [x] `handler.go:27` — ミドルウェアタイムアウト `60 * time.Second` がハードコード。環境変数で設定可能にすべき
- [x] `repository.go:174-175` — キャッシュTTL `5分` とクリーンアップ間隔 `10分` がハードコード。環境変数で設定可能にすべき
- [x] `provider_common.go:13` — HTTPクライアントタイムアウト `30秒` がハードコード。環境変数で設定可能にすべき
- [x] `service.go:26` — SourceIdentifier `"api.winget-src"` がハードコード。環境変数で設定可能にすべき

## パフォーマンス

- [x] `repository.go:114` — `QueryPackageManifests` がパッケージリストを線形探索している。`map[string]PackageListEntry` によるO(1)ルックアップに改善すべき
- [x] `repository.go:80-107` — `QueryManifest` で条件に一致する各パッケージに対して個別に `fetchVersionsCached` を呼んでいる（N+1パターン）。初回検索時のレイテンシが大きい

## コード品質（追加3）

- [x] `repository.go:68` — `dispatchProvider` エラー時のメッセージが `"unknown package provider"` だが、`entry.Provider` の値を含めるべき（`fmt.Errorf("unknown package provider: %s", entry.Provider)`）
- [x] `provider_common.go:147` — `fetchChecksums` のフォーマットエラーメッセージが `"checksum format error"` だけで、問題の行の内容が含まれていない。デバッグ困難
- [x] `gitlab.go:12-13` — `Gitlab` 構造体に `baseURL` フィールドがない。テスト時にエンドポイントを `entry.Endpoint` で渡しているが、`Github` 構造体との一貫性がない
- [x] `models.go:89` — `DataResponse.Data` の型が `interface{}` になっている。`any` に統一すべき（Go 1.18+）
- [x] `handler.go:13-18` — エラーレスポンス生成のボイラープレートが各ハンドラで重複している。ヘルパー関数に抽出すべき

## テスト（追加3）

- [x] `repository_test.go` — `NewWingetSrcRepository` に不正なYAMLや空ファイルを渡した場合のエラーテストがない
- [x] `handler_test.go` — `/packageManifests/{identifier}?Version=xxx` のクエリパラメータ付きテストがない
- [x] `provider_common_test.go` — `collectChecksums` のテストがない（チェックサムアセットが複数ある場合のマージ動作）
- [x] `provider_common.go` — `fetchChecksums` に `context.Context` が渡されていない。タイムアウト・キャンセルが効かない

## 機能追加（追加2）

- [x] Dockerfile の追加（コンテナデプロイ対応）
- [x] `packages.yaml` のサンプルファイル追加（利用者向けのクイックスタート用）
- [x] Graceful degradation — 一部プロバイダーのAPI呼び出しが失敗しても、成功したパッケージだけ返すオプション（現状は1件でもエラーなら全体失敗）

## 設定管理（追加）

- [ ] コマンドラインフラグ対応を追加し、環境変数の代替手段として使えるようにする（`main.go`）
  - `ff`（peterbourgon/ff）の利用を検討：フラグ > 環境変数 > デフォルトの優先度を自動制御できる軽量ライブラリ
  - 参考: https://github.com/peterbourgon/ff
- [ ] `token_env` フィールドを `PackageListEntry` に追加し、環境変数名でトークンを参照できるようにする（`types.go`, `github.go`, `gitlab.go`）

## セキュリティ（追加）

- [ ] TLS設定に `MinVersion: tls.VersionTLS13` を指定し、TLS 1.2 接続を禁止する（`main.go`）

## キャッシュ・パフォーマンス（追加）

- [ ] `cache.go` — エントリ数の上限（`maxEntries`）を設けてメモリ使用量を制御する
- [ ] `fetchVersionsCached` にリトライロジックを追加する（外部API一時障害への対応）（`repository.go`）

## ロギング・観測可能性（追加）

- [ ] ログレベルを環境変数（`LOG_LEVEL`）で制御できるようにする（`main.go`）
- [ ] キャッシュヒット/ミスを `slog.Debug` でログ出力する（`cache.go`）
- [ ] プロバイダー呼び出しのレイテンシをログ出力する（`repository.go`）

## テスト（追加4）

- [ ] エンドツーエンドテスト追加（`/manifestSearch` → `/packageManifests` のフロー全体を検証）
- [ ] ベンチマークテスト追加（`QueryManifest` の並行アクセス性能計測）
