# winget-src

WinGet互換のREST APIサーバー。GitHub/GitLabのリリース情報をWinGetパッケージマネージャーで利用できる形式で提供します。

## 概要

`winget-src`は、GitHub ReleasesやGitLab Releasesから取得したソフトウェアリリース情報を、Microsoft WinGetパッケージマネージャーの標準API仕様に準拠した形式で提供するRESTfulサーバーです。これにより、公式のWinGetリポジトリに登録されていないパッケージでも、WinGetを使ってインストール・管理できるようになります。

## 特徴

- **マルチプロバイダー対応**: GitHub ReleasesとGitLab Releasesに対応
- **WinGet API互換**: WinGetパッケージマネージャーの標準API仕様に準拠
- **zip-portable対応**: ポータブルアプリケーション（ZIP形式）のインストーラーをサポート
- **マルチアーキテクチャ**: x64、x86、ARM64に対応
- **チェックサム検証**: SHA256チェックサムの自動取得と検証
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

## 使い方

### 1. パッケージリストYAMLファイルの作成

パッケージ情報を定義したYAMLファイルを作成します。

```yaml
# packages.yaml
- provider: github
  id: microsoft/powertoys
  name: PowerToys
  publisher: Microsoft
  description: Windows system utilities to maximize productivity
  token: ghp_your_github_token_here  # オプション
  installer_type: zip-portable

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
| `installer_type` | ✓ | インストーラー形式（現在は `zip-portable` のみ対応） |
| `token` | - | 認証トークン（プライベートリポジトリやレート制限緩和に使用） |
| `endpoint` | - | GitLabのエンドポイント（GitLab専用、デフォルト: `https://gitlab.com`） |
| `project_id` | - | GitLabのプロジェクトID（GitLab専用） |

### 2. 環境変数の設定

```bash
export PACKAGE_LIST=/path/to/packages.yaml
export PORT=8080  # オプション、デフォルトは8080
```

### 3. サーバーの起動

```bash
./winget-src
```

サーバーが起動すると、以下のようなログが表示されます：

```
INFO start server listen
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

### `GET /information`

サーバー情報を取得します。

**レスポンス例:**
```json
{
  "Data": {
    "SourceIdentifier": "winget-src",
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
│  (データアクセス層)          │
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
- [yaml.v2](https://gopkg.in/yaml.v2) - YAML解析

### ディレクトリ構成

```
.
├── .github/
│   └── workflows/
│       └── release.yml      # GitHub Actions CI/CD
├── .goreleaser.yaml         # GoReleaser設定
├── main.go                  # エントリーポイント
├── handler.go               # HTTPハンドラー
├── service.go               # ビジネスロジック
├── repository.go            # データアクセス層
├── models.go                # WinGet APIモデル
├── types.go                 # 型定義
├── github.go                # GitHubプロバイダー
├── gitlab.go                # GitLabプロバイダー
└── go.mod                   # Go依存関係
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
