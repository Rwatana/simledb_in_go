# Chapter 1: Database Systems

## この章の問い

単なるファイル保存ではなく、なぜデータベースシステムが必要なのか。

第1章はSimpleDBの実装に入る前に、DBMSが解決すべき問題と、利用者から見た
DBエンジンの境界を整理する章である。書籍上の範囲はChapter 1（本文pp.1-14、
PDF pp.15-28）。

## 先に結論

DBMSは「大量の共有データを、遅い永続記憶の上で、壊さず、使いやすく扱う」ための
仕組みである。そこから、次の機能が必要になる。

| 要求 | 単純なファイルだけでは困ること | DBMSの仕組み | 後続章 |
| --- | --- | --- | --- |
| 永続性 | 更新のたびに巨大ファイルを書き直す | block/page単位の局所更新 | Ch.3 |
| 高速な検索 | 全件を先頭から読む | index | Ch.12 |
| 複数利用者 | 同時更新で値を失う | transaction、lock、commit | Ch.5 |
| 障害耐性 | crash時に中途半端な更新が残る | log、rollback、recovery | Ch.4-5 |
| 大容量 | disk/flashはRAMより大幅に遅い | buffer cache | Ch.4 |
| 使いやすさ | 利用者が毎回ファイル処理を書く | SQL、parser、planner | Ch.8-10 |
| 効率 | 同じ結果でも実行方法でコストが違う | query optimization | Ch.15 |

この対応関係が、以降のSimpleDBのpackage構成そのものになる。

## 1.1 Why a Database System?

### Record storage

大学DBの例は、`STUDENT`、`DEPT`、`COURSE`、`SECTION`、`ENROLL`という
種類のrecordからなる。概念上の表をテキストファイルへ直接保存するだけでは、
大きなファイルの途中を削除・更新すると後続データの書き直しが必要になり、検索も
全件走査になりやすい。

重要なのは、論理的なrecordと物理的な保存形式を分離すること。利用者は「学生」を
扱い、DBエンジンはblock、page、indexなどの形式で効率よく保存する。

### Multi-user access

同じ値を2人が読み、それぞれ更新すると、一方の変更が失われることがある。
DBMSは衝突する操作だけを待たせ、複数操作をtransactionとしてまとめる。

- `commit`: 変更を確定し、他の利用者から見える状態にする
- `rollback`: 未確定の一連の変更を取り消す

「同時実行できること」と「結果が正しいこと」の両立が目的であり、すべてを直列に
すればよいわけではない。

### Dealing with catastrophe

複数recordを更新している途中でprocessやmachineが停止すると、更新済みと未更新が
混在する。再起動後に処理を単純にやり直すと、すでに適用済みの更新を二重適用する
可能性がある。そのため、DBMSには実行中だったtransactionを特定し、安全な状態へ
戻すrecoveryが必要になる。

### Memory management

永続記憶はRAMより遅い。一方、全データをRAMに置くことはできず、RAMは電源断で
内容を失う。DBMSは使用中のpageをRAMへcacheし、永続版との同期を管理する。

ここには2つの異なる責務がある。

1. どのpageをRAMへ残し、どれを追い出すか
2. crashしても永続データを正しい状態へ戻せる順序で書くこと

### Usability

利用者が「2019年卒業の学生名」を得たいだけなのに、ファイルを開き、各行を分解し、
値を比較するコードまで毎回書くのは関心の混在である。SQLは取得したい結果を宣言し、
具体的な読み方をDBエンジンへ委ねる。

この「whatとhowの分離」はClean Architectureにも通じる。内側は達成したい操作を表し、
外側がファイルや通信の具体的な方法を担当する。

## 1.2 Derbyと1.4 SimpleDB

Derbyは実用的で多機能なJava DBMS、SimpleDBは内部構造を読むため意図的に機能と
アルゴリズムを小さくした教材である。SimpleDBの非効率さは欠陥ではなく、各componentの
責務と協調を観察しやすくする設計上の選択である。

本プロジェクトでも、最初から高性能・完全なSQL互換を目指さない。まず書籍と同じ
振る舞いを小さなテストで理解し、その後に改善を別Issueとして比較できるようにする。

## 1.3 Database Engines

database applicationは、利用者と対話するUIと、データを管理するdatabase engineに
分けられる。UIはengineのAPIを呼び、engineは保存・検索・transactionなどを担う。

接続方式は2つある。

| 方式 | engineの場所 | 特徴 | Clean Architecture上の見方 |
| --- | --- | --- | --- |
| embedded | UIと同じprocess | 単純で、applicationが専有しやすい | in-process adapter |
| server-based | 専用server process | 複数client、network越しに利用可能 | transport adapter + server composition |

接続文字列だけで方式を切り替え、利用側のSQL操作を変えなくてよい点が重要である。
これはDBの中核APIを通信方式から独立させる設計の例になっている。

編集可能な図は[DBMSの責務と接続境界](../diagrams/ch01-database-system.drawio)に置く。

## 1.5 SimpleDB SQLの境界

SimpleDBは学習目的のため、SQLを意図的に限定する。

- 基本は`select-from-where`
- `where`の結合は`and`、比較は等価のみ
- alias、rename、計算式、組み込み関数、nested queryなし
- `group by`、`order by`、outer join、union、nullなし
- insertは明示的な値のみ
- updateの代入は1つのみ

この制限は今後のテストで重要になる。「未対応の構文を偶然受け入れない」ことも
仕様の一部として扱う。

## Java版ソースリーディング

第1章は個々のclass実装を説明しておらず、詳細なソースリーディングはChapter 3から
始まる。ただし、配布版SimpleDB 3.4のpackage構成を眺めると、第1章で列挙されたDBMSの
責務がコードへ対応していることを確認できる。

| 責務 | Java package | 最初に読む候補 |
| --- | --- | --- |
| 物理ファイル | `simpledb.file` | `BlockId`, `Page`, `FileMgr` |
| cache | `simpledb.buffer` | `Buffer`, `BufferMgr` |
| log | `simpledb.log` | `LogMgr`, `LogIterator` |
| transaction | `simpledb.tx` | `Transaction`, `BufferList` |
| record | `simpledb.record` | `Schema`, `Layout`, `RecordPage` |
| query | `simpledb.query` | `Scan`, `SelectScan`, `ProductScan` |
| SQL解析 | `simpledb.parse` | `Lexer`, `Parser` |
| 実行計画 | `simpledb.plan` | `Plan`, `Planner` |
| 外部接続 | `simpledb.jdbc`, `simpledb.server` | adapter群、`SimpleDB`, `StartServer` |

### 今回読んだ入口

第1章の接続境界を確認するため、実装詳細へ深入りせず3つの入口を読んだ。

- `simpleclient/SimpleIJ.java`: connection stringに`//`があるかで`NetworkDriver`と
  `EmbeddedDriver`を選ぶ。接続後のquery/update loopは接続方式を知らない。
- `simpledb/server/SimpleDB.java`: `FileMgr`、`LogMgr`、`BufferMgr`の順で具体実装を
  組み立て、transaction、recovery、metadata、plannerを初期化する。
- `simpledb/server/StartServer.java`: `SimpleDB`を生成し、remote driverをRMI registryへ
  登録する。network transportはengineの外側に置かれている。

Go版では、この「組み立て」を`cmd`側のComposition Rootへ寄せる。Java版の`SimpleDB`
classをそのまま巨大なGo structにするかどうかは、後続章のusecaseが見えてから判断する。

読むときは「Javaのclassをどう写すか」ではなく、次をメモする。

1. その型が守る不変条件
2. 外部I/Oの有無
3. 利用側が本当に必要とする操作
4. 失敗がどう表現されるか
5. 単体テストと結合テストのどちらで確かめるべきか

## Clean Architectureへの翻訳

第1章から決められるのは依存方向だけであり、具体的なinterface一覧ではない。

- record、transactionなどの中核概念をCLIやnetwork protocolへ依存させない
- embeddedとserver-basedで同じusecaseを再利用する
- filesystem実装は外側へ置く
- parserは文字列を中核の入力へ変換するadapterとして扱う
- interfaceはテストダブルを作るためではなく、利用側が複数実装を必要としたときに置く

空のlayer packageを先に作ると、学習対象より構造の維持が目的になりやすい。
Chapter 3の最初の振る舞いから、必要な最小境界を具体化する。

## テスト観点メモ

Chapter 1自体には実装対象がないためGoテストはない。後続章で要求をテストへ変換する際は、
次の観点を引き継ぐ。

- persistence: processをまたいでも値が残るか
- concurrency: 競合する更新で値が失われないか
- atomicity: 途中失敗で一部だけ更新されないか
- recovery: crash後に説明可能な状態へ戻るか
- performance boundary: 全件走査や過剰なI/Oが起きていないか
- usability: 利用側が保存形式を知らずに操作できるか

## 自分の言葉で答えるチェックポイント

- text fileへrecordを1行ずつ保存する方法は、なぜ規模が大きくなると破綻するか
- commit前の変更を他の利用者へ見せてはいけないのはなぜか
- cacheが速さを生む一方で、どんな整合性問題を増やすか
- database engineとUIを分けると、embedded/server接続をどう差し替えやすくなるか
- Gitのcommit/working treeとDB transactionのcommit/rollbackは、どこが似ていてどこが違うか

## 次の最小Issue候補

1. Chapter 2を読み、JDBC APIを「利用者が必要とするDBエンジン境界」として整理する
2. Chapter 3の`BlockId`を読み、Goの値オブジェクトとしてテストから実装する
3. `Page`の整数・文字列エンコードを、境界値テストとともに実装する
4. 一時ディレクトリを使う`FileMgr` integration testを追加する

実装の最初のPRは候補2だけに限定し、`Page`や`FileMgr`を同時に作らない。

## 参照

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Chapter 1
- SimpleDB 3.4 source distribution, `README.txt` and `simpledb/*`
