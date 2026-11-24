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
| 4 | Memory and Log Management | 基礎実装済み | [LogManagerとBufferManagerの解説・実装記録](docs/chapters/04-log-management.md) |
| 5 | Transaction Management | 基礎実装済み | [Transactionの解説・実装記録](docs/chapters/05-transaction-management.md) |
| 6 | Record Management | 基礎実装済み | [Schema、Layout、RecordPage、TableScanの解説・実装記録](docs/chapters/06-record-management.md) |
| 7 | Metadata Management | 実装中 | [Catalog metadataの解説・実装記録](docs/chapters/07-metadata-management.md) |
| 8 | Query Processing | 実装中 | [query scanと関係演算子の解説・実装記録](docs/chapters/08-query-processing.md) |
| 9 | Parsing | 実装完了（PR stack上） | [SQL tokenizationとupdate-command dispatchの解説・実装記録](docs/chapters/09-parsing.md) |
| 10 | Planning | 実装中 | [query plan contract、TablePlan/SelectPlan/ProductPlan/ProjectPlanのscan・schema・estimateの解説・実装記録](docs/chapters/10-planning.md) |
| 11 | JDBC Interfaces | 実装中 | [server manager composition、transaction startとJava source map](docs/chapters/11-jdbc-interfaces.md) |
| 12 | Indexing | 実装中 | [index traversal contractと索引実装の解説・実装記録](docs/chapters/12-indexing.md) |
| 13 | Materialization and Sorting | 未着手 | — |
| 14 | Effective Buffer Utilization | 未着手 | — |
| 15 | Query Optimization | 未着手 | — |

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

Chapter 11 JDBC Interfacesでは、`internal/server.NewManagers`でfile・log・buffer managerを明示設定から組み立て、共有管理セットからSTART記録付きtransactionを生成します。process-restart recovery、metadata、planner、transaction lifecycle、embedded JDBC client、network clientは別の振る舞いとして後続に分けます。

Chapter 12 Indexingでは、値/RIDベースの索引走査契約、100個の静的hash bucketへのJava互換キー割当、静的hash indexの探索コスト見積もり、hashとB-treeの初期化・走査・挿入・削除、索引scan/plan、B-tree page・分割・root growthを実装しています。葉とdirectory pageを同時にpinせず、1 frameでも探索できるようにしています。

Chapter 9 ParsingのSQL tokenizationとupdate-command dispatchは完了し、Chapter 10 Planningではquery plan contract、SELECT/UPDATE planner、TablePlan・SelectPlan・ProductPlan・ProjectPlanのscan/schema/estimateを実装中です。Chapter 13 Materialization and Sorting、Chapter 14 Effective Buffer Utilization、Chapter 15 Query Optimizationは未着手です。

## 参照元

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Springer, 2020
- SimpleDB 3.4 Java source distribution

書籍PDFとJava配布物は著作物のため、このリポジトリには複製せず、各自が正規に
入手したローカル資料を参照します。
