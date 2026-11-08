# Chapter 3: Disk and File Management

## この章の進め方

Chapter 3からSimpleDBの内部実装を始める。章全体を1つのPull Requestへ入れず、利用者から
見える振る舞いごとにIssue、test、最小実装、解説を完結させる。

| Feature | 学ぶ概念 | 状態 |
| --- | --- | --- |
| `BlockID` | file内の論理blockを識別する値 | 実装済み |
| `Page` | blockと同じ大きさのmemory領域と値のencoding | 実装済み |
| `FileManager` read/write | block番号をbyte offsetへ変換するfile I/O | 未着手 |
| `FileManager` append/length | fileをblock単位で拡張・計測する操作 | 未着手 |

参考実装のstep-by-step documentationと同様に、概念、使い方、実行結果、図を実装順に残す。
このプロジェクトではさらに、各featureのtest、JavaからGoへの設計判断、意図的に含めなかった
範囲を記録する。

## Feature 1: BlockID

編集可能な図は[Draw.io版](../diagrams/ch03-block-id.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch03-block-id.svg)を参照する。

### 問い

DB file内の特定のblockを、OSのbyte位置や開いたfile handleへ依存せずにどう表すか。

### 概念

SimpleDBのdatabaseはtable、index、log、catalogなどを複数のfileへ保存する。各fileは固定長の
blockへ論理的に分割される。`BlockID`は次の2値で1つのblockを識別する。

- database fileの名前
- file内で0から数える論理block番号

例えば`student.tbl`のblock 23は次のように表す。

```go
block := storage.NewBlockID("student.tbl", 23)
```

`BlockID`はdata自体やblock sizeを持たない。将来`FileManager`がI/Oするときに、block先頭の
byte offsetを次の式で求める。

```text
byte offset = block number × block size
```

block sizeが400 bytesなら、block 23の先頭は`23 × 400 = 9200` bytesである。この計算と
filesystem I/Oは今回の`BlockID`の責務ではない。

### 公開API

| API | 責務 |
| --- | --- |
| `NewBlockID(fileName, number)` | file名と論理block番号から値を作る |
| `FileName()` | blockを含むdatabase file名を返す |
| `Number()` | 0始まりの論理block番号を返す |
| `String()` | logやdiagnostic向けに位置を説明する |

使用例:

```go
block := storage.NewBlockID("student.tbl", 23)

fmt.Println(block.FileName()) // student.tbl
fmt.Println(block.Number())   // 23
fmt.Println(block)            // [file student.tbl, block 23]
```

### なぜ値オブジェクトにするか

file名とblock番号を別々の引数として渡し続けると、組み合わせを取り違えたり、同じblockかを
比較する規則が呼び出し側へ散らばる。1つの値にすると、次の用途で同じ識別規則を共有できる。

- buffer poolでcache済みpageを探すkey
- lock tableで共有・排他lockを管理するkey
- log recordで更新対象を示す値
- record pageが現在の格納場所を示す値

`BlockID`はI/Oを行わず、OSやnetworkへ依存しない。したがってstorage engineの内側に置く。

## Test-firstの記録

### Red

最初に外部test packageから期待する使い方を書いた。実装前はpackageに非test Go fileがなく、
buildが失敗した。

```text
github.com/Rwatana/simledb_in_go/internal/storage: no non-test Go files
FAIL github.com/Rwatana/simledb_in_go/internal/storage [build failed]
```

この失敗により、testがまだ存在しない実装を本当に要求していることを確認した。

### Green

file名とblock番号だけを持つ`BlockID`と、constructor、accessor、`String`を追加した。

```text
ok github.com/Rwatana/simledb_in_go/internal/storage
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestBlockIDIdentifiesAFileBlock` | constructorへ渡した2値を取得できる |
| `TestBlockIDEqualityUsesFileNameAndNumber` | file名とblock番号の両方で同値性が決まる |
| `TestBlockIDCanBeUsedAsMapKey` | comparableな値としてcacheやlockのkeyにできる |
| `TestBlockIDStringDescribesTheLocation` | diagnosticで位置を読める |

## Javaソースリーディング

対象は`simpledb.file.BlockId`である。

| Java | Go | 判断 |
| --- | --- | --- |
| `String filename` | private `string` field | 外から書き換えさせない |
| `int blknum` | private `int` field | block番号を値の一部にする |
| `fileName()` | `FileName()` | Goのexport規則に合わせる |
| `number()` | `Number()` | 0始まりであることをdoc commentへ記す |
| `equals()` | `==` | comparableなstructなので個別methodは不要 |
| `hashCode()` | map key | Go runtimeが同じfieldからhashを扱う |
| `toString()` | `String()` | `fmt.Stringer`の慣例に合わせる |

Java版のclass構造をそのまま写さず、Goの言語機能で同じ契約を小さく表した。structのfieldを
`string`と`int`だけに保つことは、`==`で比較できmap keyにもできるというAPI上の性質を持つ。
将来sliceやmapをfieldへ追加するとこの性質が失われるため、変更時はtestが検出する。

## Clean Architecture上の判断

`BlockID`を`internal/storage`へ置いた。これはdatabase engineが使う中核の値であり、具体的な
filesystem APIを知らない。後続のfile I/O実装は`BlockID`へ依存してよいが、`BlockID`から
`os.File`やfile managerへ依存させない。

```text
filesystem adapter -> BlockID
buffer manager     -> BlockID
lock manager       -> BlockID

BlockID -X-> os.File / network / CLI
```

interfaceも追加していない。値の生成、比較、表示に外部依存を差し替える必要がないためである。

## 意図的に含めなかったもの

- file名が空かどうかのvalidation
- 負のblock番号のvalidation
- block sizeとbyte offsetの計算
- page contents
- diskへのread/write/append
- error型

書籍とJava版の`BlockId`もconstructorでvalidationを行わない。最初のPRでは教材の契約を越えて
規則を追加せず、filesystem境界で実害のある失敗をtestするときにvalidation場所を決める。

## Feature 2: Page memory encoding

編集可能な図は[Draw.io版](../diagrams/ch03-page-encoding.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch03-page-encoding.svg)を参照する。図は固定長Pageの中で4-byte整数と
length-prefixedなbyte列がどのoffsetを占めるかを示す。

### 問い

disk blockへ書き出す前の値を、固定長memory領域のどこに、何byte使って保存するか。

### 概念

`Page`は固定capacityのbyte列を持ち、値の先頭位置をoffsetで指定する。整数は必ず4 bytes、
任意のbinary payloadは4-byteのlength prefixとpayload本体として格納する。

```text
integer = 4-byte signed value
bytes   = 4-byte payload length + payload
```

複数の値を同じPageへ置く場合、呼び出し側が前の値のencoded lengthを使って次のoffsetを決める。
`Page`自身はrecord layoutやfield typeを知らず、指定された位置のencodingだけを担当する。

### 公開API

| API | 責務 |
| --- | --- |
| `NewPage(capacity)` | 固定capacityの空Pageを作る |
| `Capacity()` | 利用可能な総byte数を返す |
| `SetInt` / `GetInt` | offset指定で4-byte整数を書き読みする |
| `SetBytes` / `GetBytes` | offset指定でlength-prefixed bytesを書き読みする |
| `Snapshot()` | file I/Oへ渡せる全Page内容のcopyを返す |

`GetBytes`と`Snapshot`は内部sliceを直接返さずcopyを返す。呼び出し側が返却値を変更しても、
Page内部のencoded valueは変わらない。

### Encoding contract

整数とlength prefixはJavaの`ByteBuffer` defaultと同じbig-endianに固定した。Goの`int`は
platform依存の幅を持つため、公開APIでは幅が明確な`int32`を使う。

```go
page, err := storage.NewPage(32)
if err != nil {
    // handle invalid capacity
}

_ = page.SetInt(0, 42)
_ = page.SetBytes(8, []byte{0xde, 0xad, 0xbe, 0xef})
```

すべてのread/writeで`offset + encoded length`がcapacity内かを確認する。範囲外accessや、
Page内に収まらないlength prefixはpanicさせず、操作とoffsetを含むerrorとして返す。

## Test-firstの記録

### Red

`Page`型を追加する前に、固定capacity、整数とbyte列のround-trip、境界error、防御的copyを
外部test packageから記述した。実装前は期待するAPIが存在しないためbuildが失敗した。

```text
internal/storage/page_test.go: undefined: storage.NewPage
internal/storage/page_test.go: undefined: storage.Page
FAIL github.com/Rwatana/simledb_in_go/internal/storage [build failed]
```

### Green

固定長sliceと`encoding/binary.BigEndian`を使う最小実装を追加した。

```text
ok github.com/Rwatana/simledb_in_go/internal/storage
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestNewPageHasFixedCapacity` | capacityが固定され、負数を拒否する |
| `TestPageStoresFourByteIntegers` | 符号付きint32をbig-endianでround-tripする |
| `TestPageStoresLengthPrefixedBytes` | 空やNULを含むpayloadをlength prefix付きで保存する |
| `TestPageRejectsOutOfBoundsAccess` | encoded rangeがcapacityを越える操作をerrorにする |
| `TestPageSnapshotDoesNotExposeMutableMemory` | snapshotから内部memoryを書き換えられない |

## Javaソースリーディング

対象は`simpledb.file.Page`である。

| Java | Go | 判断 |
| --- | --- | --- |
| `ByteBuffer.allocateDirect` | 固定長`[]byte` | off-heap最適化は計測理由がないため持ち込まない |
| `getInt` / `putInt` | `GetInt` / `SetInt` | `int32`とbig-endianで4-byte contractを明示する |
| `getBytes` / `setBytes` | `GetBytes` / `SetBytes` | 同じlength prefix形式を使い、copyを返す |
| `getString` / `setString` | 未実装 | string encodingはrecord実装が要求した時点で追加する |
| `contents()` | `Snapshot()` | package外のfile I/Oへmutable sliceを公開しない |
| `BufferOverflowException`等 | `error` | 境界失敗を呼び出し側が明示的に扱えるようにする |

Java版のAPI名は参考にするが、`ByteBuffer`のmutable positionやexceptionを再現しない。
各操作はabsolute offsetを受け取り、成功時にもPage全体のcursor状態を変化させない。

## Clean Architecture上の判断

`Page`はmemory上のencoding規則だけを持ち、`os.File`やpathをimportしないため
`internal/storage`へ置く。次の`FileManager`は外側のfilesystem実装として`BlockID`と`Page`を
利用する。`Snapshot`がcopyを返すため、I/O層がPageの内部表現を保持して後から変更することもない。

## 意図的に含めなかったもの

- string専用APIと文字encoding
- `BlockID`との関連付け
- filesystemへのread/write/append
- block sizeの設定管理
- record layoutとfield type
- concurrent accessの同期

## 次の機能

次は`FileManager`のread/writeを独立したIssueとして扱う。`BlockID.Number()`とblock sizeから
file offsetを計算し、一時ディレクトリのfileと固定capacityの`Page`を往復できる振る舞いを
integration testで説明する。appendとfile lengthはさらに後続Issueへ分ける。

## 参照元

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Section 3.5 and Fig. 3.13
- SimpleDB 3.4 Java source: `simpledb.file.BlockId`, `simpledb.file.Page`
- Go standard library: `encoding/binary`
- [nakamasato/database-design-and-implementation](https://github.com/nakamasato/database-design-and-implementation)
