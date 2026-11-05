# アーキテクチャと開発ルール

## 目的

このプロジェクトでのClean Architectureは、ディレクトリを増やすことではなく、
データベースの本質的な規則をOS、CLI、ネットワーク、永続化方式の詳細から守るために使う。

## 依存ルール

依存は外側から内側へ向ける。

```text
cmd / infrastructure / adapter
              ↓
           usecase
              ↓
            domain
```

- `domain`: 値オブジェクトと、外部I/Oなしで表せる不変条件
- `usecase`: 複数のドメイン操作を組み合わせる処理と、必要な外部機能のポート
- `adapter`: SQL、CLI、ネットワーク形式とユースケースの入出力を変換
- `infrastructure`: OSファイル、時計など外部資源の具体実装
- `cmd`: 具体実装を組み立てるComposition Root

内側のパッケージは外側の具体型をimportしない。外部機能が必要なら、それを利用する
側に最小のインターフェースを定義する。

## SimpleDBの章との対応（暫定）

| Chapter | Javaパッケージ | Goで検討する境界 |
| --- | --- | --- |
| 1 | 全体像 | DBエンジンとUI、外部I/Oと中核規則 |
| 2 | `simpleclient`, `jdbc` | クライアントadapterとエンジンAPI（概念整理のみ） |
| 3 | `file` | block/pageの値とファイルI/O |
| 4 | `log`, `buffer` | ログ永続化とメモリキャッシュ |
| 5 | `tx` | transaction usecase、recovery、locking |
| 6 | `record` | schema、layout、record page |
| 7 | `metadata` | catalogとmetadata service |
| 8 | `query` | relational operatorとscan |
| 9 | `parse` | SQL parser adapter |
| 10 | `plan` | planner usecase |
| 11 | `jdbc`, `server` | 外部APIとserver adapter |
| 12-15 | `index`, `materialize`, `multibuffer`, `opt` | 高速化戦略と差し替え可能な実装 |

これは最終構成ではない。実装前のテストが境界を必要としたときにだけ分割する。
Chapter 2で確認した`Connection`や`ResultSet`の責務も、その名前のGo interfaceを先に
作る根拠にはしない。接続方式、cursor、metadataのうち、実際の利用側が必要とする操作だけを
その利用側のpackageに定義する。

## テスト方針

各機能はRed-Green-Refactorで進める。

1. 書籍の要求を、利用者から見える振る舞いとしてテスト名にする
2. 失敗する最小テストを追加する
3. テストを通す最小コードを書く
4. 重複または依存方向を改善する
5. `go test ./...` と `go vet ./...` を実行する

テストの基本分類は次のとおり。

- Unit: domainとusecaseを外部I/Oなしで検証
- Contract: portを実装するadapterが同じ振る舞いを満たすことを検証
- Integration: 一時ディレクトリを使い、実ファイルとの境界を検証
- Example: 公開APIの読み方を`Example...`で示し、ドキュメントと一緒に検証

Chapter 1には実装対象がないため、Goテストは追加しない。docs-only PRではMarkdownの
相対リンクとDraw.io XMLが開けることを確認する。

## コメントと命名

GoにはPythonやJavaのような独立した「docstring」構文はないため、Go公式の
doc commentを使う。

- exportedなpackage、type、func、const、varにはdoc commentを書く
- コメントは原則として対象名から始まる完全な文にする
- APIの「何をするか」と、制約や理由を書く。コードを日本語に言い換えるだけの説明は避ける
- package commentは、そのpackageが提供する責務と依存境界を説明する
- `gofmt`を必須とし、命名とコメントはGoogle Go Style Guideを補助基準にする

例:

```go
// BlockID identifies a block within a database file.
// Block numbers are zero-based.
type BlockID struct {
    // fields are intentionally omitted until Chapter 3
}
```

参考:

- [Go Doc Comments](https://go.dev/doc/comment)
- [Google Go Style Guide](https://google.github.io/styleguide/go/)

## 実装を小さく保つ判断基準

- 1つのPRで説明する主要な振る舞いは1つ
- 将来使うかもしれないインターフェースは作らない
- Javaのpublic class 1つをGoのpublic type 1つに対応させるとは限らない
- パフォーマンス改善は計測可能なテストまたは章の学習目標とセットにする
- 書籍との差異はPR本文と章ノートに残す

## Definition of Done

- Issueの受け入れ条件を満たす
- テストが先に振る舞いを説明している（実装PRの場合）
- `gofmt`、`go test ./...`、`go vet ./...`が成功する（Goコードがある場合）
- exported APIにdoc commentがある
- 章ノートにJava版との対応とGo版の設計判断がある
- Draw.ioを変更した場合、GitHub上の差分だけでなくdraw.ioで開いて確認する
- PR本文に`Closes #<issue>`がある
