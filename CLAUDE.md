# winget-src

WinGet互換RESTソースサーバー。GitHub/GitLabリリースをWinGetパッケージとして提供する。

## コマンド

- ビルド: `go build -o winget-src`
- テスト: `go test ./...`
- リリース: `v*` タグをpushすると GoReleaser + GitHub Actions で自動ビルド・リリース

## 実行

```bash
export PACKAGE_LIST=/path/to/packages.yaml  # 必須
export PORT=8080                            # 任意（デフォルト: 8080）
./winget-src
```

## アーキテクチャ

Handler → Service → Repository → Provider の4層構造。各層はインターフェースで分離されており、テスタビリティとプロバイダーの拡張性を確保している。

- **Handler** (`handler.go`): chi routerでHTTPリクエストを処理
- **Service** (`service.go`): ビジネスロジック（検索条件の組み立て、バージョンフィルタリング）
- **Repository** (`repository.go`): パッケージリストの管理、プロバイダーへのディスパッチ、キャッシュ
- **Provider** (`github.go`, `gitlab.go`, `provider_common.go`): 外部APIとの通信、リリース情報の取得

## 注意点

- 新しいプロバイダーを追加する場合、`PackageProvider` インターフェースを実装し `repository.go` の `dispatchProvider` にケースを追加する
- 対応インストーラータイプ: `zip-portable`, `msi`, `exe`。新タイプ追加時は `provider_common.go` にビルド関数を追加し、各プロバイダーの `FetchVersions` 内の switch 文にケースを追加する
- プロバイダー共通ロジック（アーキテクチャ検出、チェックサム収集、バージョン構築）は `provider_common.go` に集約されている

## 作業ルール

- コマンドはプロジェクトルートのカレントディレクトリで直接実行する（`cd` や `-C` フラグは使わない）
- タスク消化の都度、コミットする
