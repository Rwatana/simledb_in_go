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
| 11 | Indexing | 実装中 | [index traversal contractと索引実装の解説・実装記録](docs/chapters/11-indexing.md) |

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

Chapter 9のparsingを終え、Chapter 10 PlanningのQueryPlanner、UpdatePlanner契約、BasicQueryPlannerによるテーブル・ビューSELECT計画、BetterQueryPlannerによるブロックアクセス見積もりに基づく結合順序の選択、BasicUpdatePlannerによるDELETE・UPDATE・INSERT・CREATE TABLE・CREATE VIEW・CREATE INDEX、TablePlan/SelectPlan/ProductPlan/ProjectPlanを追加しています。Chapter 11 Indexingでは、値/RIDベースの索引走査契約、100個の静的hash bucketへのJava互換キー割当、静的hash indexの探索コスト見積もり、HashIndexの初期化、スキャン解放、キーbucketへの位置決め、衝突を除外する一致エントリー走査、現在の索引行からのRID取得、hash bucketへのエントリー挿入、指定したRIDのhash entry削除、IndexSelectScanの初期位置決め、一致行への移動、選択行の整数・文字列フィールド読み取り、query constantとしての型付き値取得、フィールド有無の確認、索引・表スキャンの解放、IndexInfoからの静的hash index生成、IndexSelectPlanのchild schema公開、scan生成、ブロックアクセス、一致行数、distinct値の見積もり、IndexJoinScanの左行キーによる初期位置決め、一致RIDの走査、整数フィールドの右優先読み取り、型付きフィールドの右優先読み取り、文字列フィールドの右優先読み取り、結合フィールドの存在確認、索引結合scanのリソース解放、IndexJoinPlanの左・右schema結合、走査生成、ブロックアクセス・出力レコード数・distinct値見積もり、B-treeページのpinning、Java互換sorted slot position、ページのclose、次レコードの容量確認、typed dataval、page flagの読み書き、B-treeページrecord countの読み書き、directory child block番号とleaf record RIDの読み取り、typed datavalの読み書き、directory child block番号の書き込みとleaf entryのRID書き込み、directory entry挿入、leaf entry挿入とB-tree page entry削除、ページ初期化、新規ページの追加と初期化、B-treeページ後半レコードの分割、directory keyによるchild block選択、child directory pageの1段階オープン、複数レベルを下ったleaf block探索、level-zero directoryからleaf pageへのpin移行、leaf page内の次slot検索と検索キー一致判定、search keyと先頭キーに基づくoverflow blockの選択、選択済みoverflow leaf pageへのpin移行、search key直前へのleaf cursor初期位置決め、一致entry・overflow pageへの次進行、cursor現在slotのRID取得、RID指定のleaf entry削除、B-tree directory entry値、空きslotへのleaf entry挿入、葉カーソルのclose、最小キーを先頭にもつleaf pageの分割、同値キーだけを持つfull leaf pageのoverflow chainと、混合キー葉ページのduplicate-group境界split、key順のdirectory entry挿入を実装しています。

## 参照元

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Springer, 2020
- SimpleDB 3.4 Java source distribution

書籍PDFとJava配布物は著作物のため、このリポジトリには複製せず、各自が正規に
入手したローカル資料を参照します。
