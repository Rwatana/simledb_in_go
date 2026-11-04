# SimpleDB in Go

『Database Design and Implementation』の教育用DBMS **SimpleDB 3.4** を題材に、
データベース内部を学びながらGoで段階的に再実装するプロジェクトです。

完成品を一度に移植するのではなく、章ごとに次のサイクルを回します。

1. 書籍で概念を学ぶ
2. Java版SimpleDBの該当ソースを読む
3. MarkdownとDraw.ioで理解を言語化・図解する
4. Issueに小さな受け入れ条件を書く
5. テストを先に置き、必要最小限のGoコードを実装する
6. Issueを閉じる小さなPull Requestとしてレビュー可能にする

## 現在地

| Chapter | テーマ | 状態 | 成果物 |
| --- | --- | --- | --- |
| 1 | Database Systems | 学習資料作成済み | [章まとめ](docs/chapters/01-database-systems.md) / [概念図](docs/diagrams/ch01-database-system.drawio) |
| 2 | JDBC | 未着手 | Goにおけるクライアント境界を整理予定 |
| 3 | Disk and File Management | 未着手 | 最初のGo実装候補 |

第1章は概念導入の章であり、書籍もSimpleDBのソースコード解説を第3章から
始めています。そのため、第1章のPRにはGo実装を含めません。

## 設計方針

- Clean Architectureの依存方向を守る
- 章の説明に必要になるまで抽象化やパッケージを追加しない
- Javaのクラス構造を機械的に写さず、Goの小さなパッケージとインターフェースへ翻訳する
- インターフェースは実装側ではなく、利用側が必要になった時点で定義する
- 公開APIにはGoのdoc commentを付け、Google Go Style Guideを補助的な基準にする
- 正常系だけでなく、境界値・永続化・障害時の振る舞いをテスト名で説明する

詳細は[アーキテクチャ方針](docs/architecture.md)を参照してください。

## リポジトリ構成（予定）

```text
.
├── cmd/                    # CLIやサーバなどのComposition Root
├── internal/
│   ├── domain/             # OSや通信に依存しない値・規則
│   ├── usecase/            # DB操作のユースケース
│   ├── adapter/            # CLI・SQL・通信との変換
│   └── infrastructure/     # ファイルシステムなど外部I/O
├── docs/
│   ├── chapters/           # 章ごとの学習ノート
│   └── diagrams/           # 編集可能なDraw.io図
└── README.md
```

この構成は先に空ディレクトリを作らず、実装する機能が要求した時点で追加します。

## Issue / Pull Requestの単位

1つのIssueは、レビュー可能な1つの学習成果または振る舞いに限定します。

- Issue: 背景、学習対象、受け入れ条件、対象外を記載
- Branch: `type/<issue-number>-<short-description>`形式にする
- Commit: 1 Pull Request = 1 Commitとし、修正はamendする
- Pull Request: 本文に `Closes #<issue>` を書き、学び・実装・テスト結果を残す

詳しい規約とCIの検査内容は[Contributing](CONTRIBUTING.md)を参照してください。

## 次の一歩

Chapter 2はJava固有のJDBCが中心です。Goへ直接移植せず、DBエンジンの利用者側に
必要な境界を読み解く学習Issueとして扱います。最初の実装はChapter 3の
`BlockID`相当の値オブジェクトから始める予定です。

## 参照元

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Springer, 2020
- SimpleDB 3.4 Java source distribution

書籍PDFとJava配布物は著作物のため、このリポジトリには複製せず、各自が正規に
入手したローカル資料を参照します。
