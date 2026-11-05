# Chapter 2: JDBCからDBクライアント境界を学ぶ

## この章の位置づけ

Chapter 2の対象は書籍15〜47ページ（手元のPDFでは29〜61ページ）である。
この章ではDB内部のデータ構造をまだ実装せず、Javaの標準APIであるJDBCを通して、
アプリケーションとDBエンジンの間にどのような境界が必要かを学ぶ。

この章で答える問いは次の3つである。

1. DBの利用者は、接続から結果の解放までをどの責務として扱うか
2. Embedded接続とNetwork接続で、変わるものと変わらないものは何か
3. JDBCの設計をGoへ翻訳するとき、今は何を実装しないべきか

結論は、JDBCのクラス構造をそのままGoへ移植しないことである。Chapter 2では
利用者から見える責務とリソース寿命だけを記録し、具体的なGo APIは実際のユースケースが
必要になった時点で設計する。

## 全体像

編集可能な図は[Draw.io版](../diagrams/ch02-client-boundary.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch02-client-boundary.svg)を参照する。

JDBCはベンダー固有の接続方法を、共通のクライアントAPIから分離する。アプリケーションは
同じ`Connection`、`Statement`、`ResultSet`を使い、EmbeddedとNetworkの違いは主に
`Driver`と接続文字列、およびその先に置かれるadapterが引き受ける。

## 基本となる5つのJDBCインターフェース

| インターフェース | 利用者から見た責務 | SimpleDBでの役割 |
| --- | --- | --- |
| `Driver` | 接続文字列を解釈し、接続を作る | EmbeddedまたはNetwork実装を選ぶ |
| `Connection` | DBとのセッションとtransactionを所有する | `Statement`作成、commit、rollback、close |
| `Statement` | SQLをDBへ渡して実行する | queryまたはupdateをplannerへ渡す |
| `ResultSet` | query結果を1行ずつ読む | `Scan`をcursorとして包む |
| `ResultSetMetaData` | 列名、型、表示幅などを説明する | `Schema`をクライアント向けに変換する |

### 接続境界

書籍のSimpleDBでは次の接続文字列を使う。

```text
jdbc:simpledb:studentdb     # 同一process内のEmbedded接続
jdbc:simpledb://localhost   # RMIを使うNetwork接続
```

接続文字列の解釈と具体的な接続の生成は`Driver`が担当する。その後のquery実行と結果の
読み方は共通インターフェースで表される。この分離により、アプリケーションの中心的な処理は
接続方式を知る必要がない。

### 実行境界

`Statement`には、結果の形が異なる2つの実行方法がある。

- `executeQuery`: 行を返し、`ResultSet`を取得する
- `executeUpdate`: 変更した行数を整数で返す

queryは結果を読み終えるまでDB側の`Scan`やtransactionを保持する。一方、SimpleDBの
updateは実行後にcommitされる。APIの戻り値だけでなく、処理中に保持する資源と解放時点を
合わせて理解する必要がある。

### 結果cursor

`ResultSet`は結果全体の配列ではなく、現在行を指すcursorである。

```text
before first --next()--> row 1 --next()--> ... --next()--> after last
```

cursorは最初の行より前から始まる。`next()`が`true`を返した間だけ、`getInt`や
`getString`で現在行を読む。SimpleDBでは`close()`が内部の`Scan`を閉じ、queryに対応する
transactionをcommitする。

Javaのtry-with-resourcesに相当する構文はGoにはない。将来Go APIを設計するときは、
`defer rows.Close()`のように所有者と解放責務が読み取れる形、またはcallback内だけcursorを
有効にする形を比較する。

### 結果のメタデータ

`ResultSetMetaData`は列数、列名、JDBC型、表示幅をquery結果から取得する。これにより
`SimpleIJ`はSQLごとの列構成を事前に知らなくても、表形式で結果を表示できる。

JDBCの列番号は1から始まる。Goで同じAPIを作る理由はないため、将来の設計では番号による
アクセスを本当に必要とするか、列名やiteratorで十分かを利用側のテストから判断する。

## リソースとエラーの寿命

接続、結果cursor、transactionは外部資源または状態を保持する。利用側が`close`を忘れると、
file、lock、network connection、server側のcursorが残る可能性がある。

Chapter 2から得る設計上の制約は次のとおり。

- 資源を作った層が、解放方法を利用者へ明示する
- queryの完了条件を「SQLを送信した時」ではなく「結果を閉じた時」まで含めて考える
- adapter内部の例外を、境界で扱える安定したerrorへ変換する
- rollbackが必要な失敗と、入力や通信の失敗を将来区別できる余地を残す
- ただし、実際の利用例がない段階でerror型の階層を先に作らない

Java版では内部の`RuntimeException`やRMIの`RemoteException`を`SQLException`へ変換する。
これは、外側の技術的な詳細を利用者向けのerror境界へ写すadapterの仕事と捉えられる。

## transactionとautocommit

一般的なJDBCでは、初期状態のautocommitにより1つのSQL文が1つのtransactionになる。
queryは`ResultSet`が閉じられたときに完了する。複数のSQL文を原子的に扱う場合は
autocommitを無効にし、`commit`または`rollback`を明示する。

```java
conn.setAutoCommit(false);
try {
    // multiple statements
    conn.commit();
} catch (SQLException error) {
    conn.rollback();
    throw error;
}
```

ただし、SimpleDB 3.4のadapterは`setAutoCommit`を実装していない。書籍が説明する
一般的なJDBC機能と、教材DBが実際に提供する機能を混同しないことが重要である。

## transaction isolation

JDBCが定義する主なisolation levelと、防ごうとする現象を整理する。

| Level | dirty read | non-repeatable read | phantom |
| --- | --- | --- | --- |
| Read Uncommitted | 許す | 許す | 許す |
| Read Committed | 防ぐ | 許す | 許す |
| Repeatable Read | 防ぐ | 防ぐ | 実装により許す |
| Serializable | 防ぐ | 防ぐ | 防ぐ |

- dirty read: 別transactionの未commitデータを読む
- non-repeatable read: 同じ行を再読したとき値が変わる
- phantom: 同じ検索条件を再実行したとき行の集合が変わる

SimpleDB 3.4の`ConnectionAdapter`では`setTransactionIsolation`も未実装である。
isolationの実装はlockingとtransactionを学ぶChapter 5以降の課題として残す。

## PreparedStatement

一般的なJDBCの`PreparedStatement`は、SQL中の値を`?`として分離し、型ごとのsetterで
値を設定する。同じSQLを再利用しやすく、DBが準備済みの実行計画を活用できる場合がある。
値とSQL構文を分けることは、文字列連結による引用符の誤りを避けるうえでも重要である。

```java
PreparedStatement statement = conn.prepareStatement(
    "select sname from student where majorid = ?"
);
statement.setInt(1, majorId);
```

SimpleDB 3.4は`prepareStatement`を実装していない。そのため教材clientの一部は入力値を
SQL文字列へ連結している。これは移植すべき設計ではなく、教材実装の機能制限として記録する。

## clientとDBのどちらで計算するか

原則として、選択、結合、集約などDBが実行できる処理はSQLとしてDBへ渡す。DB側で処理すると、
転送する行を減らし、optimizerが利用可能なindexや実行順序を選べる。

client側で独自に全件を取得して処理する設計は、次の場合にだけ検討する。

- DBが表現できない処理である
- 計測により、転送量やDB負荷を含めても有利だと確認できる
- 一貫性とtransaction境界が明確である

Chapter 2の段階では最適化を実装せず、「計算場所もadapter境界の設計判断である」と記録する。

## Javaソースリーディング

### 対応表

| 読んだ場所 | 観察した責務 |
| --- | --- |
| `simpledb.jdbc.embedded.EmbeddedDriver` | URLからDB名を取り出し、同一processの接続を作る |
| `EmbeddedConnection` | `SimpleDB`、現在の`Transaction`、`Planner`を所有する |
| `EmbeddedStatement` | plannerへquery/updateを渡し、失敗時にrollbackする |
| `EmbeddedResultSet` | `Scan`をJDBC cursorとして包み、close時にcommitする |
| `EmbeddedMetaData` | `Schema`をJDBCの列情報へ変換する |
| `simpledb.jdbc.network.NetworkDriver` | URLからhostを取り出し、RMI registryへ接続する |
| `NetworkConnection`ほか | `Remote*`呼び出しをJDBC APIへ変換するclient adapter |
| `Remote*Impl` | server側でplanner、transaction、scanを所有する |
| `simpledb.server.StartServer` | RMI registryを起動し、remote driverを公開する |
| `simpleclient.SimpleIJ` | metadataを使って任意query結果を表示する |
| `simpleclient.StudentMajorNoJDBC` | JDBCなしでPlanner、Plan、Scanを直接使う |

### Embeddedの呼び出し経路

```text
Application
  -> EmbeddedDriver
  -> EmbeddedConnection
  -> EmbeddedStatement
  -> Planner / Transaction
  -> Plan / Scan
  -> EmbeddedResultSet
```

全て同一process内で呼ばれる。`ResultSet`が`Scan`を隠し、applicationにDB内部型を
公開しない点がadapterとして重要である。

### Networkの呼び出し経路

```text
Application
  -> NetworkDriver / NetworkConnection / NetworkStatement
  -> Remote* interface
  -> RMI transport
  -> Remote*Impl
  -> Planner / Transaction / Scan
```

Network版ではclient側とserver側に対称的なwrapperが置かれる。通信は外側の詳細であり、
plannerやtransactionの規則にRMI型を持ち込まない。

### 一般的なJDBCとSimpleDB 3.4の差

SimpleDBの`*Adapter`はJDBCの大きなinterfaceを満たすための基底classであり、多くのmethodは
`operation not implemented`を返す。具体classが上書きするのは教材に必要な小さなsubsetだけである。

| 機能 | 書籍での説明 | SimpleDB 3.4 |
| --- | --- | --- |
| 基本query/update | あり | 対応 |
| metadata | あり | 対応 |
| explicit commit/rollback | あり | Embeddedの一部で対応 |
| autocommit切替 | あり | 未対応 |
| isolation level切替 | あり | 未対応 |
| `PreparedStatement` | あり | 未対応 |
| scrollable/updatable result | あり | 未対応 |

この差は欠陥として隠すのではなく、教材を小さく保つ意図と実装上の制約として残す。

## Clean Architectureへの翻訳

Chapter 2から得られる概念的な境界は次のとおりである。

| 利用者の必要 | 将来検討するGoの責務 | 依存方向 |
| --- | --- | --- |
| DB操作の開始・終了 | session/connection | applicationからusecaseへ |
| commandの実行 | query/update request | adapterからusecaseへ変換 |
| 行の逐次取得 | rows/cursor/iterator | engine内部の`Scan`を隠す |
| 列の説明 | result schema | domain schemaを外部表現へ変換 |
| remote接続 | transport adapter | networkから内側へ変換 |

これはGoの型名を決めた表ではない。Clean Architectureでは、interfaceは具体実装の都合ではなく
利用側が必要とする操作として定義する。現時点では利用するGo applicationが存在しないため、
`Connection`や`Statement`というinterfaceを先に追加しない。

### 今回実装しないもの

- JDBCを模倣したGo package
- network serverと通信protocol
- transactionとisolation level
- SQL parser、planner、optimizer
- prepared statement
- 公開DB APIとerror hierarchy

Chapter 3からfileとblockの小さな振る舞いを実装し、その機能を実際に使う層が現れた時点で
必要最小限のportを定義する。

## 将来のテスト観点

Chapter 2ではGoコードを追加しないが、将来クライアント境界を実装するときのcontract候補を残す。

- query cursorは最初の行より前から始まる
- 最後の行を越えた後は値を読めない
- closeは複数回呼んでも資源を残さない
- query中の失敗では必要なrollbackと資源解放が行われる
- embeddedとnetwork adapterは同じ利用者向けcontractを満たす
- adapterは内部の`Scan`やtransport固有errorを公開しない
- updateの戻り値は変更行数として一貫する

これらは実装Issueの受け入れ条件へ移すときに、失敗するテストから始める。

## セルフチェック

1. `ResultSet`をsliceではなくcursorとして扱う理由を説明できるか
2. queryのtransactionが`executeQuery`直後に完了しない理由を説明できるか
3. EmbeddedとNetworkでapplication側の処理が共通になる理由を説明できるか
4. JavaのJDBC interfaceを今Goへ作らない理由を説明できるか
5. 一般的なJDBC機能とSimpleDB 3.4の実装範囲を区別できるか

## 次の一歩

Chapter 3ではdiskとfile managementを読み、最初の実装単位として`BlockID`相当の値を扱う。
最初のIssueは、file名と0始まりのblock番号を持つ値の不変条件、等価性、文字列表現を
テストで説明する最小単位にする。

## 参照元

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Chapter 2
- SimpleDB 3.4 Java source: `simpledb.jdbc`, `simpledb.server`, `simpleclient`
- [JDBC Basics](https://docs.oracle.com/javase/tutorial/jdbc/basics/)

