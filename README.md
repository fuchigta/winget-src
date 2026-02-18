# winget-src

WinGet互換のREST APIサーバー。GitHub/GitLabのリリース情報をWinGetパッケージマネージャーで利用できる形式で提供します。

## 概要

`winget-src`は、GitHub ReleasesやGitLab Releasesから取得したソフトウェアリリース情報を、Microsoft WinGetパッケージマネージャーの標準API仕様に準拠した形式で提供するRESTfulサーバーです。これにより、公式のWinGetリポジトリに登録されていないパッケージでも、WinGetを使ってインストール・管理できるようになります。

## 特徴

- **マルチプロバイダー対応**: GitHub ReleasesとGitLab Releasesに対応
- **WinGet API互換**: WinGetパッケージマネージャーの標準API仕様に準拠
- **マルチインストーラー対応**: zip-portable（ZIPポータブル）、msi、exeのインストーラー形式をサポート
- **マルチアーキテクチャ**: x64、x86、ARM64に対応
- **チェックサム検証**: SHA256チェックサムの自動取得と検証
- **キャッシュ機能**: バージョン情報をメモリキャッシュして外部APIへのリクエストを削減
- **Graceful Degradation**: 一部プロバイダーの失敗時も他の成功結果を返すオプション
- **軽量**: Go言語で実装された高速なマイクロサービス
- **クロスプラットフォーム**: Windows、Linux、macOS向けビルド対応

## 必要要件

- Go 1.21.4以降（ビルドする場合）
- パッケージリストYAMLファイル

## インストール

### バイナリのダウンロード

[Releases](https://github.com/fuchigta/winget-src/releases)ページから、お使いのプラットフォーム向けのバイナリをダウンロードしてください。

### ソースからビルド

```bash
git clone https://github.com/fuchigta/winget-src.git
cd winget-src
go build -o winget-src
```

### GoReleaserを使用したビルド

```bash
goreleaser build --snapshot --clean
```

### Dockerを使用したビルド

```bash
docker build -t winget-src .
```

## 使い方

### 1. パッケージリストYAMLファイルの作成

パッケージ情報を定義したYAMLファイルを作成します。`packages.yaml.example`を参考にしてください。

```yaml
# packages.yaml
- provider: github
  id: microsoft/powertoys
  name: PowerToys
  publisher: Microsoft
  description: Windows system utilities to maximize productivity
  token: ghp_your_github_token_here  # オプション
  installer_type: zip-portable

- provider: github
  id: example/app
  name: Example App
  publisher: Example
  description: An example application
  installer_type: msi

- provider: gitlab
  id: gitlab-org/gitlab-runner
  name: GitLab Runner
  publisher: GitLab
  description: GitLab CI/CD Runner
  endpoint: https://gitlab.com
  project_id: 250833
  token: glpat_your_gitlab_token_here  # オプション
  installer_type: zip-portable
```

#### YAML設定フィールド

| フィールド | 必須 | 説明 |
|-----------|------|------|
| `provider` | ✓ | プロバイダー種別（`github` または `gitlab`） |
| `id` | ✓ | パッケージID（GitHubの場合: `owner/repo`、GitLabの場合は任意） |
| `name` | ✓ | パッケージ名 |
| `publisher` | ✓ | 発行者名 |
| `description` | ✓ | パッケージの説明 |
| `installer_type` | ✓ | インストーラー形式（`zip-portable`、`msi`、`exe`） |
| `token` | - | 認証トークン（プライベートリポジトリやレート制限緩和に使用） |
| `endpoint` | - | GitLabのエンドポイント（GitLab専用、デフォルト: `https://gitlab.com`） |
| `project_id` | - | GitLabのプロジェクトID（GitLab専用） |

### 2. 環境変数の設定

```bash
export PACKAGE_LIST=/path/to/packages.yaml  # 必須
export PORT=8080                            # オプション、デフォルト: 8080
```

#### オプションの環境変数

| 環境変数 | デフォルト | 説明 |
|---------|-----------|------|
| `CACHE_TTL` | `5m` | バージョン情報キャッシュの有効期限 |
| `CACHE_CLEANUP_INTERVAL` | `10m` | 期限切れキャッシュの定期削除間隔 |
| `HTTP_CLIENT_TIMEOUT` | `30s` | 外部APIへのHTTPリクエストのタイムアウト |
| `HANDLER_TIMEOUT` | `60s` | HTTPハンドラー全体のタイムアウト |
| `GRACEFUL_DEGRADATION` | `false` | `true` の場合、一部プロバイダー失敗時も他の成功結果を返す |
| `SOURCE_IDENTIFIER` | `api.winget-src` | WinGet API の SourceIdentifier フィールド値 |

### 3. サーバーの起動

```bash
./winget-src
```

サーバーが起動すると、以下のようなログが表示されます：

```
INFO start server listen
```

### Dockerでの起動

```bash
docker run -e PACKAGE_LIST=/app/packages.yaml \
  -v /path/to/packages.yaml:/app/packages.yaml \
  -p 8080:8080 \
  winget-src
```

### 4. WinGetからの利用

WinGetの設定ファイル（`settings.json`）にカスタムソースを追加します。

```json
{
  "experimentalFeatures": {
    "experimentalMSStore": true
  },
  "sources": [
    {
      "name": "custom",
      "arg": "http://localhost:8080",
      "type": "Microsoft.PreIndexed.Package"
    }
  ]
}
```

その後、WinGetコマンドでパッケージを検索・インストールできます：

```powershell
# パッケージ検索
winget search --source custom

# パッケージインストール
winget install --id microsoft/powertoys --source custom
```

## APIエンドポイント

### `GET /health`

ヘルスチェック。サーバーが正常に動作しているか確認します。

**レスポンス:** `200 OK`

### `GET /information`

サーバー情報を取得します。

**レスポンス例:**
```json
{
  "Data": {
    "SourceIdentifier": "api.winget-src",
    "ServerSupportedVersions": ["1.0.0"]
  }
}
```

### `POST /manifestSearch`

パッケージを検索します。

**リクエストボディ:**
```json
{
  "Query": {
    "KeyWord": "powertoys"
  }
}
```

**レスポンス例:**
```json
{
  "Data": {
    "Manifests": [
      {
        "PackageIdentifier": "microsoft/powertoys",
        "PackageName": "PowerToys",
        "Publisher": "Microsoft",
        "Versions": [
          {
            "PackageVersion": "0.70.0"
          }
        ]
      }
    ]
  }
}
```

### `GET /packageManifests/{identifier}`

特定のパッケージの詳細情報を取得します。

**パラメータ:**
- `identifier`: パッケージID
- `Version`: バージョン番号（クエリパラメータ、オプション）

**レスポンス例:**
```json
{
  "Data": {
    "PackageIdentifier": "microsoft/powertoys",
    "Versions": [
      {
        "PackageVersion": "0.70.0",
        "DefaultLocale": {
          "PackageLocale": "en-US",
          "Publisher": "Microsoft",
          "PackageName": "PowerToys",
          "ShortDescription": "Windows system utilities to maximize productivity"
        },
        "Installers": [
          {
            "InstallerType": "zip",
            "Architecture": "x64",
            "InstallerUrl": "https://github.com/microsoft/PowerToys/releases/download/v0.70.0/PowerToys-0.70.0-x64.zip",
            "InstallerSha256": "abc123..."
          }
        ]
      }
    ]
  }
}
```

## アーキテクチャ

```
┌─────────────────┐
│  WinGet Client  │
└────────┬────────┘
         │ HTTP
         ↓
┌─────────────────────────────┐
│  WingetSrcHandler           │
│  - GET  /health             │
│  - GET  /information        │
│  - POST /manifestSearch     │
│  - GET  /packageManifests   │
└─────────────┬───────────────┘
              │
┌─────────────┴───────────────┐
│  WingetSrcService           │
│  (ビジネスロジック)          │
└─────────────┬───────────────┘
              │
┌─────────────┴───────────────┐
│  WingetSrcRepository        │
│  (データアクセス層・キャッシュ)│
└─────────────┬───────────────┘
              │
      ┌───────┴───────┐
      ↓               ↓
┌──────────┐   ┌──────────┐
│  GitHub  │   │  GitLab  │
│ Provider │   │ Provider │
└────┬─────┘   └────┬─────┘
     │              │
     ↓              ↓
 GitHub API    GitLab API
```

## 開発

### 依存関係

- [chi/v5](https://github.com/go-chi/chi) - HTTPルーター
- [yaml.v3](https://gopkg.in/yaml.v3) - YAML解析

### ディレクトリ構成

```
.
├── .github/
│   └── workflows/
│       └── release.yml         # GitHub Actions CI/CD
├── .goreleaser.yaml            # GoReleaser設定
├── Dockerfile                  # コンテナイメージビルド設定
├── main.go                     # エントリーポイント
├── handler.go                  # HTTPハンドラー
├── service.go                  # ビジネスロジック
├── repository.go               # データアクセス層・キャッシュ管理
├── models.go                   # WinGet APIモデル
├── types.go                    # 型定義
├── cache.go                    # 汎用キャッシュ実装
├── github.go                   # GitHubプロバイダー
├── gitlab.go                   # GitLabプロバイダー
├── provider_common.go          # プロバイダー共通関数
├── packages.yaml.example       # パッケージ設定サンプル
└── go.mod                      # Go依存関係
```

### テストの実行

```bash
go test ./...
```

### リリース

`v*`形式のタグをプッシュすると、GitHub Actionsが自動的にビルドとリリースを行います。

```bash
git tag v1.0.0
git push origin v1.0.0
```

## セキュリティ

### 認証トークン

- **GitHub**: `token`フィールドに個人アクセストークン（PAT）を設定
  - レート制限の緩和（未認証: 60リクエスト/時、認証済み: 5000リクエスト/時）
  - プライベートリポジトリへのアクセス
- **GitLab**: `token`フィールドにプライベートトークンを設定
  - ヘッダー: `PRIVATE-TOKEN`

### 注意事項

- トークンはYAMLファイルに平文で保存されるため、ファイルのパーミッションに注意してください
- 本番環境では、環境変数や秘密管理サービスの利用を推奨します

## トラブルシューティング

### `PACKAGE_LIST is required`エラー

環境変数`PACKAGE_LIST`が設定されていません。YAMLファイルのパスを指定してください。

```bash
export PACKAGE_LIST=/path/to/packages.yaml
```

### パッケージが見つからない

1. YAMLファイルの`id`フィールドが正しいか確認
2. GitHubの場合: `owner/repo`形式になっているか
3. トークンが正しく設定されているか（プライベートリポジトリの場合）
4. リポジトリにリリースが存在するか

### レート制限エラー

GitHub APIはレート制限があります。トークンを設定して制限を緩和してください。

### パッケージ取得が遅い

`CACHE_TTL`と`CACHE_CLEANUP_INTERVAL`を調整してキャッシュの有効期間を延ばすことで、外部APIへのリクエスト頻度を減らせます。

## ライセンス

このプロジェクトのライセンスについては、リポジトリのオーナーにお問い合わせください。

## 貢献

プルリクエストやイシューを歓迎します！

## 作者

fuchigta <fuchigta@alpha.co.jp>

## 関連リンク

- [WinGet公式ドキュメント](https://learn.microsoft.com/ja-jp/windows/package-manager/winget/)
- [WinGet REST API仕様](https://github.com/microsoft/winget-cli-restsource)
- [GitHub Releases API](https://docs.github.com/ja/rest/releases/releases)
- [GitLab Releases API](https://docs.gitlab.com/ee/api/releases/)
