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
