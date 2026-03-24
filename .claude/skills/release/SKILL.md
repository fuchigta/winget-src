---
name: release
description: >-
  This skill should be used when the user asks to "リリースしてください",
  "タグ打ってプッシュ", "バージョンを上げてリリース", "release",
  "bump version and push", "CIを監視して", "CI確認して",
  "リリース結果を確認", "watch CI", "monitor CI",
  or wants to publish a new version of winget-src.
  Guides through the full release workflow: determine next version,
  create a git tag, push, then monitor the Release workflow and fix failures if needed.
version: 0.1.0
---

# winget-src リリース手順

## 概要

winget-src のリリースは以下の順序で行う:

1. 現状確認（未コミットの変更、最新タグ、差分コミット）
2. 次バージョンを決定
3. `git tag v<version>` でタグ作成
4. `git push origin v<version>` でタグを push
5. GitHub Actions の `release` ワークフロー完了を確認
6. 失敗時: 分析・修正・再リリース
7. 成功確認 & リリース URL 報告

## 重要な制約

- **バージョン形式**: semver（例: `0.9.3`, `1.0.0`）
- **バージョンファイルは不要**: Go モジュールは git タグがバージョンを兼ねる。bump スクリプトは存在しない。
- **CI監視の制約**: 修正→再リリースは最大2回まで。インフラ起因の失敗は `gh run rerun --failed` で再実行（回数制限なし）。

## 手順

### Step 1: 現状確認

```bash
git status
git log --oneline -5
git tag --sort=-v:refname | head -3
```

未コミットの変更の有無と現在の最新タグを把握する。未コミットの変更がある場合はコミットしてからリリースする。

### Step 2: 次バージョンを決定する

```bash
git log <latest_tag>..HEAD --oneline
```

コミット内容を確認し、semver のルールに従って次バージョンを自動決定する。

| コミット内訳 | バージョン判定 | 確認 |
|---|---|---|
| `fix:` / `chore:` / `test:` / `docs:` / `refactor:` のみ | パッチ +1 | **不要** |
| `feat:` を含む | マイナー +1 | **不要** |
| `BREAKING CHANGE` を含む | メジャー +1 | **不要** |
| 判断が難しい・複数ルールが競合する | — | `AskUserQuestion` で確認 |

判断できる場合はそのまま Step 3 へ進む。判断できない場合のみ `AskUserQuestion` ツールで確認を取る。

### Step 3: タグ作成 & push

```bash
git tag v<version>
git push origin v<version>
```

### Step 4: Release ワークフローを監視

タグ push により GitHub Actions の `release.yml` ワークフローが起動する。**トークン消費を最小化するため、監視スクリプトを1回のツール呼び出しで実行する。**

```bash
# timeout: 600000 を指定すること（Bash ツールの上限 = 10分）
bash "<SKILL_BASE_DIR>/scripts/wait-workflows.sh"
```

- 終了コード `0`: 成功 → Step 5 へ
- 終了コード `1`: タイムアウト（まだ実行中）→ 同じコマンドを再実行
- 終了コード `2`: 失敗検出 → Step 5 へ

### Step 5: 結果判定

ワークフローが完了したら結果を確認する:

```bash
gh run view <run-id> --log-failed
```

| 状態 | 起因 | 対応 |
|---|---|---|
| `success` | — | Step 6a（完了報告）へ |
| `failure` でログにネットワーク/タイムアウト/rate-limit | インフラ起因 | `gh run rerun --failed <run-id>` で再実行（Step 4 へ戻る） |
| `failure` でログにコンパイルエラー/テスト失敗/設定ミス | コード起因 | Step 6b（修正 & 再リリース）へ |

### Step 6a: 成功確認 & リリース URL 報告

```bash
gh release view --repo fuchigta/winget-src v<version>
```

GitHub Release が作成されていることを確認し、リリース URL をユーザーに報告して完了。

### Step 6b: 修正 & 再リリース（最大2回）

コード起因の失敗の場合、修正してパッチバージョンで再リリースする。

1. 原因を特定・修正
2. 修正をコミット（`fix: ...`）
3. Step 2 に戻り次バージョン（パッチ +1）を決定
4. Step 3〜5 を実行

リトライ回数を記録し、2回目の失敗後は Step 7 へ進む。

### Step 7: リトライ上限到達

修正 & 再リリースを2回試みても失敗した場合、以下をユーザーに報告して終了する:

- 失敗したワークフローの Run ID とログ URL
- 失敗の概要（エラーメッセージ抜粋）
- 推奨アクション（手動調査 or 追加コンテキストの提供依頼）
