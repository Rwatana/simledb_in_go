# Chapter 4: Memory and Log Management

## この章の進め方

Chapter 4ではChapter 3のfixed-size file layerを使い、recovery logとbuffer管理を段階的に実装する。
logの振る舞いを小さく分け、初期化、1 block内append、flush、block rollover、1 block iteration、
previous block traversalをそれぞれ独立したIssue、test、1 commit、stacked Pull Requestとして扱う。

| Feature | 学ぶ概念 | 状態 |
| --- | --- | --- |
| log block initialization | tail pageとboundary | 実装済み |
| record append in one block | right-to-left encodingとLSN | 実装済み |
| explicit flush | durability boundary | 実装済み |
| block rollover | full tailから次blockへの遷移 | 実装済み |
| current-block iterator | 1 block内のnewest-to-oldest traversal | 実装済み |
| previous-block traversal | block境界を越えるreverse traversal | 未着手 |

## Feature 1: Log block initialization

編集可能な図は[Draw.io版](../diagrams/ch04-log-block-initialization.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch04-log-block-initialization.svg)を参照する。図は最初のlog blockでoffset 0に
boundaryを置き、残りをrecord用の空き領域として初期化する構造を示す。

### 問い

log recordをまだ1件も書いていない状態を、再openしても同じ意味になるdisk layoutでどう表すか。

### 概念

log blockの先頭4 bytesは、最も新しいrecordの開始位置を表す`boundary`である。空blockでは
recordが存在しないため、boundaryをblock sizeと同じ値にする。

```text
offset 0..3          offset 4..blockSize-1
+------------------+----------------------+
| boundary = size  | zero-filled free area|
+------------------+----------------------+
```

recordは後続PRでblock右端から左向きに格納する。初期boundaryがblock末尾を指すことで、最初の
record位置を`boundary - encoded record length`として計算できる。

### 公開API

| API | 責務 |
| --- | --- |
| `log.NewManager(files, fileName)` | existing tailをloadするか、最初のempty log blockを作る |

constructorはlog fileのlogical lengthを取得する。0 blocksならPageをzero valueで作り、offset 0へ
block sizeを書き、FileManagerの`Append`と`Write`でblock 0を永続化する。existing fileでは最後の
blockだけをreadし、新しいblockは追加しない。

## Test-firstの記録

### Red

missing fileのinitial blockとexisting tailのreopenを外部test packageから記述した。実装前は
log packageに非test Go fileがないためbuildが失敗した。

```text
github.com/Rwatana/simledb_in_go/internal/log: no non-test Go files
FAIL github.com/Rwatana/simledb_in_go/internal/log [build failed]
```

### Green

tail Page、current BlockID、FileManager参照だけを持つ最小Managerを追加した。

```text
ok github.com/Rwatana/simledb_in_go/internal/log
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestNewManagerInitializesTheFirstLogBlock` | block 0、initial boundary、zero-filled free area |
| `TestNewManagerLoadsExistingTailWithoutAppending` | reopen時にtailをloadしblockを増やさない |
| `TestNewManagerRejectsInvalidInitialization` | boundaryを格納できないblock sizeとinvalid pathを拒否する |

## Javaソースリーディング

対象は`simpledb.log.LogMgr`のconstructorと`appendNewBlock`の初期化部分である。

| Java | Go | 判断 |
| --- | --- | --- |
| `new Page(byte[])` | `storage.NewPage` | empty tail用のfixed-capacity Pageを作る |
| `fm.length(logfile)` | `files.Length(fileName)` | missing fileを0 blocksとして分岐する |
| `fm.append(logfile)` | `files.Append(fileName)` | 最初のzero-filled blockを作る |
| `setInt(0, blockSize)` | 同じboundary layout | Java iteratorと同じdisk contractを維持する |
| constructor内のunchecked exception | `(*Manager, error)` | initialization failureを呼び出し側へ返す |

existing tailのboundaryが`[4, blockSize]`外なら、破損状態としてconstructorで拒否する。Java版は
この検査を行わないが、invalid boundaryを保持したまま後続appendへ進むより原因を早く報告できる。

## Clean Architecture上の判断

`internal/log`はlogの配置規則を持ち、外側のfilesystem adapterを利用する。現時点では
`FileManager`を抽象化するinterfaceを作らない。integration testで実fileのcontractを確認でき、
差し替える利用側がまだ存在しないためである。

## 意図的に含めなかったもの

- log record appendとLSN
- public flush API
- full blockからのrollover
- reverse iterator
- buffer poolとwrite-ahead logging

## Feature 2: Record append within one block

編集可能な図は[Draw.io版](../diagrams/ch04-log-record-append.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch04-log-record-append.svg)を参照する。図はpayload 3 bytesのrecordを
32-byte blockへappendし、boundaryが32から25へ移動する例を示す。

### 問い

最新recordから逆順に読みやすくするため、複数recordを1つのfixed-size blockへどう配置するか。

### 概念

各recordはPageの`SetBytes`と同じ4-byte length prefix + payloadで表す。現在のboundaryからencoded
lengthを引いた位置へrecordを置き、その位置を新しいboundaryとしてoffset 0へ保存する。

```text
record position = boundary - (4 + payload length)
new boundary    = record position
```

recordが増えるほどboundaryは左へ移動する。iteratorはboundaryから開始してrecordのencoded length分
右へ進むことで、最新から古い順に同一blockを読める。

### 公開API

| API | 責務 |
| --- | --- |
| `Manager.Append(record)` | current tailへrecordをcopyし、新しいLSNを返す |
| `LSN` | process lifetime内でappend順を表す単調増加番号 |

この機能を導入した段階の`Append`はdiskへwriteしない。current blockに収まらない場合もrolloverせず、
Page、boundary、LSNを変更しないerrorとして返した。後述のFeature 4がこのcapacity不足経路だけを
拡張し、flushとrolloverの契約を独立してtestする。

## Test-firstの記録

### Red

2 recordsのLSNとlayout、capacity error、disk未反映を先にtestした。実装前は`Append` methodがなく
buildが失敗した。

```text
manager.Append undefined (type *Manager has no field or method Append)
FAIL github.com/Rwatana/simledb_in_go/internal/log [build failed]
```

### Green

Managerへmutexとlatest LSNを追加し、current Pageだけを更新する最小実装を追加した。

```text
ok github.com/Rwatana/simledb_in_go/internal/log
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestAppendStoresRecordsRightToLeftAndAllocatesLSN` | layout、boundary、LSN、empty/binary payload、copy |
| `TestAppendDoesNotChangeStateWhenRecordDoesNotFit` | capacity errorがPageとLSNを変更しない |
| `TestAppendDoesNotFlushTheTailPage` | appendだけではdisk boundaryを変更しない |

## Javaソースリーディング

対象は`simpledb.log.LogMgr.append`のうち、current blockにrecordが収まる経路だけである。

| Java | Go | 判断 |
| --- | --- | --- |
| `int latestLSN` | `LSN uint64` | process lifetime内の単調増加を明示する |
| `recsize + Integer.BYTES` | `len(record) + 4` | Pageのlength prefix contractを共有する |
| `boundary - bytesneeded` | 同じright-to-left計算 | reverse traversal用disk layoutを維持する |
| synchronized `append` | Manager mutex | PageとLSNの更新を1 critical sectionにする |
| full blockでflush/rollover | error | rolloverを独立PRに分離する |

入力recordは`Page.SetBytes`がcopyするため、append後に呼び出し側がsliceを変更してもtailは変わらない。

## Clean Architecture上の判断

LSN割り当てとrecord layoutはlogの中核規則なので`internal/log`に置く。filesystemへ依存するflushは
まだ呼ばず、in-memory state transitionだけをunit testで固定する。

## 意図的に含めなかったもの

- explicit flushとlast saved LSN
- capacity不足時のblock rollover
- reverse iterator
- restart後のLSN復元
- transactionとwrite-ahead logging

## Feature 3: Explicit flush through an LSN

編集可能な図は[Draw.io版](../diagrams/ch04-log-flush.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch04-log-flush.svg)を参照する。図はmemory tailにLSN 1と2があり、disk tailが
まだemptyな状態から`Flush(1)`でPage全体が保存されるdurability boundaryを示す。

### 問い

呼び出し側が指定したlog recordと、それより前のrecordがdiskへ到達したことをどう保証するか。

### 概念

Managerは`latestLSN`に加えて`lastSavedLSN`を持つ。requested LSNがまだ保存されていなければ、
current tail Page全体をFileManagerでwriteする。

```text
requested LSN <= lastSavedLSN  -> no-op
lastSavedLSN < requested LSN <= latestLSN -> write tail Page
requested LSN > latestLSN      -> error
```

Page単位でwriteするため、LSN 1を要求した時点ですでにLSN 2も同じtailにあれば、両方がdiskへ保存され、
`lastSavedLSN`は2へ進む。saved stateはFileManagerのwriteとsyncが成功した後にだけ更新する。

### 公開API

| API | 責務 |
| --- | --- |
| `Manager.Flush(lsn)` | requested LSNを含むcurrent tailをdiskへ保存する |

### Test-firstの記録

#### Red

memory/disk boundaryの差、saved LSN、future LSNを先にtestした。実装前は`Flush`と
`lastSavedLSN`が存在せずbuildが失敗した。

```text
manager.Flush undefined (type *Manager has no field or method Flush)
manager.lastSavedLSN undefined
FAIL github.com/Rwatana/simledb_in_go/internal/log [build failed]
```

#### Green

Manager mutex内でLSNを検査し、必要な場合だけFileManagerへtail Pageをwriteする実装を追加した。

```text
ok github.com/Rwatana/simledb_in_go/internal/log
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestFlushPersistsTheTailThroughRequestedLSN` | requested LSNを含むPage全体を保存しlatestまでsavedにする |
| `TestFlushRejectsFutureLSNAndSkipsSavedLSN` | future LSNのerrorとsaved以下のno-op |
| `TestFlushZeroOnEmptyLogIsNoOp` | empty logのLSN 0はPageを変えない |

## Javaソースリーディング

対象は`simpledb.log.LogMgr.flush(int)`とprivate `flush()`である。

| Java | Go | 判断 |
| --- | --- | --- |
| `lsn >= lastSavedLSN` | `lsn > lastSavedLSN` | 同じsaved LSNの不要なrewriteを避ける |
| `fm.write(currentblk, logpage)` | `files.Write(block, page)` | FileManagerのsync済みblock writeを再利用する |
| `lastSavedLSN = latestLSN` | 同じ | Page writeで同時に保存された最新recordまで進める |
| 未割当future LSN | error | 呼び出し側のLSN誤りをsilentに受け入れない |

## Clean Architecture上の判断

flushの「どのLSNまで保存済みか」という規則はlog Managerが持ち、実際のsyncはfilesystem adapterへ
委譲する。ManagerはOS file handleを直接扱わない。

## 意図的に含めなかったもの

- capacity不足時のautomatic flushとblock rollover
- iterator開始時のautomatic flush
- data Pageとのwrite-ahead logging連携
- crash後のLSN復元

## Feature 4: Full block rollover

編集可能な図は[Draw.io版](../diagrams/ch04-log-block-rollover.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch04-log-block-rollover.svg)を参照する。図はfull memory tailから、durableなold
blockと初期化済みnew blockを経て、new recordをmemoryだけに持つtailへ切り替わる順序を示す。

### 問い

current tailにrecordが入らないとき、old recordsを失わず、new blockも再open可能なdisk stateにしてから
appendを続けるには、どの順でstateを変えるべきか。

### 概念

recordのencoded lengthがempty blockのrecord領域を超える場合は、I/Oの前にrejectする。収まるrecordが
current tailにだけ入らない場合は、次の順でrolloverする。

```text
1. old tail Pageをwriteし、old latest LSNまでsavedにする
2. zero-filled blockをfile末尾へappendする
3. new Pageのboundaryをblock sizeにし、その初期状態をwriteする
4. Managerのcurrent PageとBlockIDをnew tailへ切り替える
5. recordをnew tailのmemory Pageへappendし、新しいLSNを割り当てる
```

手順3でempty stateをdiskへ保存するため、new blockが存在するのにboundary 0という無効な中間状態を
残さない。手順5のrecordは通常appendと同じくmemory-onlyであり、明示`Flush`または次回rolloverまで
durableではない。

### 公開API

公開APIは増やさない。`Manager.Append(record)`のcapacity不足経路だけを拡張する。privateな
`rollOverLocked`と`flushTailLocked`は、1つのmutex critical section内でstate transitionを組み立てる。

### Test-firstの記録

#### Red

20-byte blockを最初のencoded recordで満たし、2件目がblock 1へ移ることを先にtestした。実装前は
従来どおりcapacity errorで停止した。

```text
Append(second) rollover error = append log record of 4 bytes to
[file simpledb.log, block 0]: encoded length 8 does not fit before boundary 4
FAIL github.com/Rwatana/simledb_in_go/internal/log
```

#### Green

empty block容量の事前検査と、old-tail flush、initialized block append、tail切替を追加した。

```text
ok github.com/Rwatana/simledb_in_go/internal/log
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestAppendRollsOverAFullTailBlock` | old tail durability、new block initialization、連続LSN、memory-only new record |
| `TestAppendRejectsARecordThatCannotFitAnEmptyBlockBeforeRollover` | oversized recordをI/O前にrejectしPage、BlockID、LSN、lengthを維持 |

## Javaソースリーディング

対象は`simpledb.log.LogMgr.append`のfull-block分岐と`appendNewBlock`である。

| Java | Go | 判断 |
| --- | --- | --- |
| `flush()` | `flushTailLocked(latestLSN)` | old tailをnew block作成前にdurableにする |
| `fm.append(logfile)` | `files.Append(fileName)` | zero-filled blockをfile末尾へ追加する |
| `setInt(0, fm.blockSize())` | 同じinitial boundary | iteratorとreopenが解釈できるempty layoutを維持する |
| `fm.write(currentblk, logpage)` | `files.Write(block, page)` | record追加前のvalid empty blockをdiskへ保存する |
| oversized recordの明示検査なし | I/O前のcapacity error | 無駄なflushやempty block追加を防ぐ |

## Clean Architecture上の判断

rolloverの順序とdurability boundaryはlog policyとして`internal/log`が持つ。block追加、Page write、syncの
仕組みは引き続きfilesystem adapterへ委譲する。public helperを増やさず、appendという1 use caseの
atomicなmutex区間に限定した。

## 意図的に含めなかったもの

- log recordを読むiterator API
- previous blockへのreverse traversal
- iterator開始時のautomatic flush
- restart後のLSN復元
- transactionとwrite-ahead logging

## Feature 5: Current-block reverse iterator

編集可能な図は[Draw.io版](../diagrams/ch04-current-block-iterator.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch04-current-block-iterator.svg)を参照する。図はiterator作成時のPage snapshotと、
boundaryからblock末尾へ進むことでLSN 3、2、1の順にpayloadを返す流れを示す。

### 問い

right-to-leftで格納した可変長recordを、writerや後続appendの影響を受けずにnewest-to-oldestで読むには
どの位置から、どれだけ進めばよいか。

### 概念

`Manager.Iterator`はmutex内でcurrent Pageをcopyし、snapshotのboundaryを最初のpositionにする。
`Iterator.Next`はpositionのlength-prefixed bytesを読み、次の式でoldest側へ進む。

```text
record = page.GetBytes(position)
position = position + 4 + len(record)
position == block size ならexhausted
```

recordはright-to-leftにappendされるため、最小offsetのboundaryが最新recordであり、offsetを増やすほど
古くなる。Pageの`GetBytes`はpayloadをcopyするので、callerが返り値を変更しても別iteratorのsnapshotを
壊さない。

### 公開API

| API | 責務 |
| --- | --- |
| `Manager.Iterator()` | current in-memory tailのsnapshotから1 block iteratorを作る |
| `Iterator.Next()` | 次に古いrecordと存在有無を返す |

この段階ではiterator作成時にflushしない。snapshot後にManagerへappendされたrecordも既存iteratorには
現れない。disk traversalとprevious block移動を次のPRへ分離するためである。

### Test-firstの記録

#### Red

binary payload、empty payload、snapshot後append、exhaustion、copy isolationを先にtestした。実装前は
`Manager.Iterator`が存在せずbuildが失敗した。

```text
manager.Iterator undefined (type *Manager has no field or method Iterator)
FAIL github.com/Rwatana/simledb_in_go/internal/log [build failed]
```

#### Green

current Page snapshotとpositionだけを持つone-block Iteratorを追加した。

```text
ok github.com/Rwatana/simledb_in_go/internal/log
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestIteratorReadsCurrentBlockNewestToOldest` | reverse order、binary/empty payload、snapshot、exhaustion、no flush |
| `TestIteratorSnapshotsReturnedRecordsAndEmptyTail` | empty blockと独立したpayload copy |

## Javaソースリーディング

対象は`simpledb.log.LogIterator`のconstructor、`moveToBlock`、`next`の1 block内経路である。

| Java | Go | 判断 |
| --- | --- | --- |
| FileManagerからcurrent blockをread | memory Pageのsnapshot | flushとdisk traversalを後続PRに分離する |
| `currentpos = boundary` | `position = boundary` | 最新recordから開始する |
| `currentpos += 4 + rec.length` | 同じ | length prefixを含めて古いrecordへ進む |
| `Iterator<byte[]>` | `Next() ([]byte, bool, error)` | exhaustionとdecode errorをGoの戻り値で表す |
| block末尾でprevious blockへ移動 | 今回はexhausted | cross-block traversalを独立した振る舞いにする |

## Clean Architecture上の判断

iteratorはlogのdisk layoutを解釈するため`internal/log`に置く。current featureはimmutable Page snapshotだけを
所有し、FileManagerへ依存しない。readerとwriterのlock共有時間をPage copyの間だけに制限する。

## 意図的に含めなかったもの

- previous blockへの移動
- iterator作成時のautomatic flush
- disk Pageのread failure
- concurrent live view
- transaction recoveryとの統合

## 次の機能

次はcurrent blockを読み終えたiteratorがprevious blockへ移動する振る舞いだけを実装する。
iterator作成時のautomatic flushは、さらに別Issue、別commit、別stacked PRへ分ける。

## 参照元

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Chapter 4
- SimpleDB 3.4 Java source: `simpledb.log.LogMgr`
