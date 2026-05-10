package handler

type demoNote struct {
	Name    string
	Content string
}

func demoWelcomeNotes() []demoNote {
	return []demoNote{
		{
			Name: "Welcome!.md",
			Content: `# Welcome to GophDrive!

This is a demo of GophDrive, a serverless Markdown note-taking application.

## Key Features
- **Self-Hosted Storage** Notes are stored in your own DynamoDB table
- **Serverless** Built on AWS Lambda for high availability
- **WebAssembly** Core logic runs in your browser via Go-compiled Wasm
- **Offline Support** View and edit your notes without an internet connection

## Markdown Syntax
Here are some examples of Markdown elements you can use:

### Text Decoration
- **Bold text** for emphasis
- *Italic text* for subtle emphasis
- ~~Strikethrough~~ to indicate removed content
- ` + "`" + `Inline code` + "`" + ` for technical terms

### Tables
| Feature | Status | Description |
| :--- | :--- | :--- |
| Preview | Active | Fast rendering via WebAssembly |
| Sync | Active | Optimistic concurrency via ETag |
| Search | Active | Server-side substring search |

### Lists
- Item 1
- Item 2
    - Nested item
- Item 3

### Checklists
- [x] Open the app
- [ ] Write a note
- [ ] Save it

### Code Blocks
` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello, GophDrive!")
}
` + "```" + `

### Blockquotes
> This is a blockquote. Perfect for highlighting important information.

---
Enjoy exploring GophDrive!`,
		},
		{
			Name: "ようこそ!.md",
			Content: `# GophDrive へようこそ！

これは、サーバーレスなマークダウンノートアプリのデモです。

## 主な特徴
- **セルフホスト** ノートは自分のDynamoDBテーブルに保存されます
- **サーバーレス** AWS Lambda 上で動作
- **WebAssembly** コアロジック（マークダウン変換等）は Go で書かれ、ブラウザ上で動作
- **オフライン対応** インターネットがなくてもノートの閲覧・編集が可能

## マークダウン記法

### テキスト装飾
- **太字** による強調
- *斜体* による控えめな強調
- ~~打ち消し線~~ による削除の表現
- ` + "`" + `インラインコード` + "`" + ` による技術用語の記述

### テーブル
| 機能 | 状態 | 備考 |
| :--- | :--- | :--- |
| プレビュー | 有効 | WebAssemblyによる高速表示 |
| 同期 | 有効 | ETagによる楽観的同時実行制御 |
| 検索 | 有効 | サーバ側の部分一致検索 |

### リスト
- 項目 1
- 項目 2
    - ネストされた項目
- 項目 3

### チェックリスト
- [x] アプリを開く
- [ ] ノートを書く
- [ ] 保存する

### コードブロック
` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello, GophDrive!")
}
` + "```" + `

### 引用
> これは引用文です。重要な情報を強調するのに適しています。

---
さあ、自由にノートを作成して GophDrive を体験してみてください！`,
		},
	}
}
