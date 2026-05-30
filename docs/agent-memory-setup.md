# AI エージェント外部記憶セットアップ

GophDrive を Claude Code の外部記憶として使うための手順書。

## 概要

`gophmem` CLI が GophDrive REST API を叩き、Claude Code が `~/.claude/skills/gophdrive-memory.md` の Skill に従って記録・参照する。エージェントはあなたの Google アイデンティティ（同一 `sub`）で動作し、Vault の `AI Memory` フォルダに書き込む。

## 前提

- GophDrive が本番デプロイ済み（`./scripts/deploy-aws.sh` 完了）
- GophDrive に Google ログイン済みであること
- `gophmem` バイナリがビルド済み（下記「CLI インストール」参照）

---

## 1. CLI インストール

```bash
cd tools/gophmem
go build -o gophmem .
# PATH の通った場所に置く（例）
mv gophmem ~/bin/gophmem
```

ローカル開発時は `GOPHMEM_BASE_URL=http://localhost:3000/api` が自動で使われる。

---

## 2. API キーを発行する

1. ブラウザで GophDrive にアクセスし Google ログイン
2. 右上のメニューから **Settings** を開く
3. **API Keys** セクションの「**Issue Key**」ボタンをクリック
4. 表示された平文キーをコピー（このダイアログを閉じると再表示されません）

---

## 3. gophmem の環境変数を設定する

```bash
# ~/.zshrc や ~/.bashrc に追記
export GOPHMEM_BASE_URL=https://<your-cloudfront-domain>/api
export GOPHMEM_API_KEY=<手順 2 でコピーしたキー>
```

---

## 4. 動作確認

```bash
# AI Memory フォルダを作成し最初のノートを書く
gophmem write "howto: agent memory セットアップ完了" --tags agent,ops

# 一覧確認
gophmem list

# 検索確認
gophmem search "セットアップ"
```

Web UI の `AI Memory` フォルダにノートが表示されれば成功。

---

## ローカル開発での設定

`.env` ファイル（`.env.example` を参照）に追記:

```bash
API_KEY_HASHES_TABLE=APIKeyHashes
```

ローカルでは `gophmem` のデフォルト URL が `http://localhost:8080` を指すため `GOPHMEM_BASE_URL` は省略できる。
（Next.js は静的エクスポートのため `:3000` は API プロキシを持たない。バックエンドの `:8080` に直接アクセスする。）

API キーの発行はローカルの GophDrive UI から行う（手順 2 と同様）。

---

## キーのローテーション・失効

Settings の **API Keys** セクションから即座に操作できます。

- **Regenerate Key**: 旧キーを即時失効させ、新しいキーを発行します（DynamoDB 上でアトミックに置換）。
- **Revoke**: キーを失効させます。再度使うには「Issue Key」が必要です。

> **SSM 方式との違い**: 旧方式は Lambda の cold start まで旧キーが有効でした。新方式は DynamoDB ルックアップのため **即時失効**します。

---

## Skill のインストール

Claude Code は `~/.claude/skills/` ディレクトリ内の `.md` ファイルを Skill として読み込む。`gophdrive-memory.md` は既にこのリポジトリの `tools/gophmem/` 管理外の `~/.claude/skills/` に配置済み（セットアップスクリプトで自動インストール予定）。

Skill の使い方は `~/.claude/skills/gophdrive-memory.md` を参照。

---

## トラブルシューティング

| 症状 | 原因 | 対処 |
|------|------|------|
| `gophmem write` で 401 | `GOPHMEM_API_KEY` が誤り | Settings > API Keys でキーを Regenerate してコピーし直す |
| `gophmem write` で 401（ローカル） | キーが未発行またはローカル UI で発行していない | ローカルの GophDrive UI から「Issue Key」して再設定 |
| `gophmem write` で 403 | demo アカウントでログイン中 | Google ログインで本アカウントに切り替える |
| `AI Memory` フォルダが見つからない | 初回作成中にエラー | `gophmem list` を再実行。`~/.cache/gophmem/folders.json` を削除して再試行 |
