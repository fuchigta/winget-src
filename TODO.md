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
