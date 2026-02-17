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
