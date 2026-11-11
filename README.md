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
| 2 | JDBC | 学習資料作成済み | [章まとめ](docs/chapters/02-jdbc.md) / [編集用Draw.io](docs/diagrams/ch02-client-boundary.drawio) / [表示用SVG](docs/diagrams/ch02-client-boundary.svg) |
| 3 | Disk and File Management | 基礎実装済み | [BlockID、Page、FileManagerの解説・実装記録](docs/chapters/03-disk-and-file-management.md) |
| 4 | Memory and Log Management | 実装中 | [LogManagerの解説・実装記録](docs/chapters/04-log-management.md) |

第1章は概念導入、第2章はJDBCによるクライアント境界の説明であり、書籍もSimpleDBの
内部実装の解説を第3章から始めています。そのため、第1章と第2章のPRにはGo実装を含めません。

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
- Title: IssueとPull Requestに`[Docs]`、`[Go]`、`[CI/CD]`のprefixを付ける
- Branch: `feature/#<issue-number>_<snake_case_description>`形式にする
- Commit: 1 Pull Request = 1 Commitとし、修正はamendする
- Pull Request: `Issue Number`を明記し、本文に`Closes #<issue>`を書く

詳しい規約とCIの検査内容は[Contributing](CONTRIBUTING.md)を参照してください。

## 次の一歩

Chapter 3の基礎file layerと、Chapter 4のlog manager基本機能が揃いました。buffer管理は未割当frame、
pin increment、unpin、modifying transaction、recovery LSN、最初のBlockID割当、WAL順flush、clean frameの
再割当、dirty frameのWAL順replacement、fixed-size BufferManager pool初期化、existing assignment lookupまで
進み、最初のunpinned victim候補を選び、cache済みframeをpinして0→1遷移だけをavailable countへ反映できます。
次はManager経由のunpinと1→0遷移だけを、独立したIssueとPRで追加します。

## 参照元

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Springer, 2020
- SimpleDB 3.4 Java source distribution

書籍PDFとJava配布物は著作物のため、このリポジトリには複製せず、各自が正規に
入手したローカル資料を参照します。
