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
| previous-block traversal | block境界を越えるreverse traversal | 実装済み |
| iterator creation flush | disk traversal前のtail durability | 実装済み |
| unassigned buffer frame | fixed-size cache Pageの初期状態 | 実装済み |
| buffer pin increment | frame利用開始と0→1遷移 | 実装済み |
| buffer unpin | frame利用終了とunderflow防止 | 実装済み |
| dirty transaction | frameを変更したtransactionの識別 | 未着手 |
| dirty buffer flush | WAL順序とdata Page保存 | 未着手 |
| block assignment | frame再利用 | 未着手 |
| buffer pool | frame選択と待機 | 未着手 |

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

この機能を導入した段階ではiterator作成時にflushしなかった。snapshot後にManagerへappendされたrecordも
既存iteratorには現れない。disk traversalとdurability契約を個別にtestするためであり、後述のFeature 7が
iterator作成前のflushだけを追加する。

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

## Feature 6: Previous-block traversal

編集可能な図は[Draw.io版](../diagrams/ch04-previous-block-traversal.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch04-previous-block-traversal.svg)を参照する。図はmemory snapshotのblock 2を読み
終えた後、disk block 1、block 0をlazy loadしてglobal reverse orderを維持する流れを示す。

### 問い

1 block iteratorのpositionがblock末尾に達したとき、log全体のnewest-to-oldest orderを壊さずに
どのblockとoffsetへ移動すべきか。

### 概念

IteratorはPageとpositionに加えてcurrent BlockIDとFileManager参照を持つ。positionがPage capacityと
一致し、block numberが0より大きければ、numberを1減らしたblockをreadし、そのPageのboundaryを
新しいpositionにする。

```text
while position == page capacity:
    if block number == 0: exhausted
    block = block number - 1
    page = read(block)
    position = page boundary
```

`while`にすることで、validだがemptyなblockが途中にあってもさらに前へ進める。readまたはboundary
validationに失敗した場合、Iteratorのcurrent stateを進めずerrorを返す。

### 公開API

公開APIは変えない。`Iterator.Next()`のblock末尾経路だけを拡張する。この機能を導入した段階ではcurrent
tailをManagerのmemory snapshotから読み、previous blocksはrollover時に保存済みのdisk Pageをlazy read
した。後述のFeature 7がsnapshot前のtail flushだけを追加する。

### Test-firstの記録

#### Red

3 blocksへ4 recordsを配置し、global reverse orderとinvalid previous boundaryを先にtestした。実装前は
current blockの1 recordを返した直後にexhaustedとなった。

```text
Next() record 1 ok = false, want true
Next(corrupt previous block) = ([], false, <nil>), want boundary error
FAIL github.com/Rwatana/simledb_in_go/internal/log
```

#### Green

IteratorへFileManagerとBlockIDを追加し、block末尾でprevious Pageをloadするprivate transitionを追加した。

```text
ok github.com/Rwatana/simledb_in_go/internal/log
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestIteratorTraversesPreviousBlocksNewestToOldest` | 3 blocksのglobal reverse order、block 0 exhaustion、no flush |
| `TestIteratorReportsAnInvalidPreviousBlockBoundary` | previous Pageをcommitする前のboundary validationとerror |

## Javaソースリーディング

対象は`simpledb.log.LogIterator.hasNext`、`next`のblock切替、`moveToBlock`である。

| Java | Go | 判断 |
| --- | --- | --- |
| `currentpos < blockSize || blk.number() > 0` | `Next`内のposition/block判定 | separate HasNextによるI/O error隠蔽を避ける |
| `blk.number()-1` | 同じprevious BlockID | file内の物理reverse orderを維持する |
| `fm.read(blk, p)` | `files.Read(block)` | previous blockだけを必要時にloadする |
| `currentpos = boundary` | `position = blockBoundary(...)` | 各blockの最新recordから再開する |
| boundary検査なし | `[4, capacity]`検査 | corrupt Pageをrecordとしてdecodeしない |

## Clean Architecture上の判断

cross-block iteratorはlog layoutとblock sequenceを解釈する`internal/log`の責務であり、raw filesystem操作は
FileManagerへ委譲する。previous Pageはeager copyせずlazy readし、iterator 1つのmemory使用量を1 Pageに
保つ。

## 意図的に含めなかったもの

- iterator作成時のautomatic flush
- 全previous Pageのeager snapshot
- iteration中の外部file mutationに対するsnapshot isolation
- restart後のLSN復元
- transaction recoveryとの統合

## Feature 7: Flush before iterator creation

編集可能な図は[Draw.io版](../diagrams/ch04-iterator-creation-flush.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch04-iterator-creation-flush.svg)を参照する。図はunsaved current tailをdiskへwriteし、
saved LSNを進めた後に同じPageからiterator snapshotを作るdurability boundaryを示す。

### 問い

recovery readerがlog iterationを開始した時点で、current tailを含む全recordがdiskから再読可能であることを
どう保証するか。

### 概念

`Manager.Iterator`はManager mutexを保持したまま`flushTailLocked(latestLSN)`を呼び、その成功後に
current Pageをcopyする。

```text
unsaved latest LSN
    -> write and sync current Page
    -> lastSavedLSN = latestLSN
    -> copy Page into iterator snapshot
    -> return Iterator
```

flushが失敗すればsnapshotを返さない。saved LSN以下またはempty logでは既存のprivate flush helperが
no-opにするため、不要なrewriteを増やさない。

### 公開API

signatureは変えず、`Manager.Iterator() (*Iterator, error)`の作成前conditionを強化する。`error`はPageの
decodeだけでなく、tail write/sync failureも表す。

### Test-firstの記録

#### Red

2 recordsをmemoryだけへappendし、Iterator作成後のsaved LSNとdisk Page一致を先にtestした。実装前は
saved LSNが0のままで、disk current blockもinitial boundaryのままだった。

```text
lastSavedLSN = 0, want latest LSN 2
disk current block after Iterator() differs from the iterator source Page
FAIL github.com/Rwatana/simledb_in_go/internal/log
```

#### Green

Iterator作成のmutex区間冒頭で、既存の`flushTailLocked`を再利用した。

```text
ok github.com/Rwatana/simledb_in_go/internal/log
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestIteratorFlushesTheCurrentTailBeforeSnapshot` | saved LSN、disk/current Page一致、iterator作成成功 |
| `TestIteratorTraversesPreviousBlocksNewestToOldest` | current blockもflushされた状態でcross-block orderを維持 |

## Javaソースリーディング

対象は`simpledb.log.LogMgr.iterator`とprivate `flush`である。

| Java | Go | 判断 |
| --- | --- | --- |
| `iterator()`冒頭の`flush()` | `flushTailLocked(latestLSN)` | readerを返す前にtailをdurableにする |
| `fm.write(currentblk, logpage)` | `files.Write(block, page)` | sync済みFileManager contractを再利用する |
| `lastSavedLSN = latestLSN` | 同じ | Page単位で保存された最新recordまで進める |
| unchecked failure | `(*Iterator, error)` | flush failure時にiteratorを返さない |
| diskからcurrent blockを再read | memory snapshot | 同じmutex区間のPage copyで余分なreadを避ける |

## Clean Architecture上の判断

「readerを公開する前にtailをdurableにする」というpolicyはlog Managerが持ち、実際のwrite/syncは
FileManagerへ委譲する。public `Flush`を再入可能にせず、同じmutex内でprivate helperを呼ぶことで
deadlockを避ける。

## 意図的に含めなかったもの

- buffer poolとreplacement policy
- data Pageのwrite-ahead logging
- restart後のLSN復元
- transaction recovery record
- concurrent file mutationに対するfull snapshot isolation

## Feature 8: Unassigned buffer frame

編集可能な図は[Draw.io版](../diagrams/ch04-unassigned-buffer.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch04-unassigned-buffer.svg)を参照する。図はFileManagerのblock sizeを使って
zero-filled Pageを所有し、まだBlockIDを持たないfresh Bufferの境界を示す。

### 問い

buffer poolへframeを並べる前に、disk I/Oやpin policyを混ぜず、各frameが必ず有効なfixed-size Pageを
所有する初期状態をどう作るか。

### 概念

`Buffer`はFileManager、LogManager、mutable Page、optional BlockIDを保持する。constructorはdependencyを
検査し、FileManagerのblock sizeと同じcapacityのzero-filled Pageを1つ作る。fresh frameのBlockIDは
未割当である。

```text
New(files, logs)
    -> Page(capacity = files.BlockSize())
    -> block = unassigned
```

`Contents`はowned Pageを返す。上位のrecord/transaction layerがPageを更新する必要があるため、ここでは
copyではなくmutable referenceを公開する。dirty trackingは後続PRで追加する。

### 公開API

| API | 責務 |
| --- | --- |
| `buffer.New(files, logs)` | zero-filled unassigned frameを作る |
| `Buffer.Contents()` | frameが所有するmutable Pageを返す |
| `Buffer.Block()` | assigned BlockIDと存在有無を返す |

### Test-firstの記録

#### Red

external test packageからcapacity、zero-fill、unassigned state、mutable contents、nil dependenciesを先に
記述した。実装前はbuffer packageにnon-test Go fileがなくbuildに失敗した。

```text
github.com/Rwatana/simledb_in_go/internal/buffer: no non-test Go files
FAIL github.com/Rwatana/simledb_in_go/internal/buffer [build failed]
```

#### Green

Page ownershipとoptional BlockIDだけを持つ最小Bufferを追加した。

```text
ok github.com/Rwatana/simledb_in_go/internal/buffer
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestNewCreatesAnUnassignedZeroFilledFrame` | capacity、zero-fill、unassigned Block、contents access |
| `TestNewRejectsNilDependencies` | FileManager/LogManager dependency validation |

## Javaソースリーディング

対象は`simpledb.buffer.Buffer`のfieldとconstructor、`contents`、`block`である。

| Java | Go | 判断 |
| --- | --- | --- |
| `new Page(fm.blockSize())` | `storage.NewPage(files.BlockSize())` | file blockと同じfixed capacityを使う |
| `BlockId blk = null` | optional `*storage.BlockID` | unassigned stateをzero-value BlockIDと混同しない |
| `contents()` | `Contents()` | 上位layerがPageを更新できるmutable reference |
| `block()`がnullable | `(BlockID, bool)` | nilをpublic APIへ漏らさない |
| constructor dependency無検査 | nil error | setup failureを早く明示する |

## Clean Architecture上の判断

BufferはPageとBlockIDというstorage valueを所有し、外側のFileManager/LogManagerを後続のI/O use case用に
参照する。初期化PRでは依存を保存するだけでI/Oを行わず、frame stateとallocation policyを分離する。

## 意図的に含めなかったもの

- pin/unpin count
- modifying transactionとLSN
- dirty Page flush
- BlockID assignmentとdisk read
- BufferManager poolとreplacement policy

## Feature 9: Increment buffer pin count

編集可能な図は[Draw.io版](../diagrams/ch04-buffer-pin.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch04-buffer-pin.svg)を参照する。図はpin count 0、1、2の遷移と、BufferManagerが
availabilityを減らすべき0→1だけが`true`になる契約を示す。

### 問い

同じframeを複数clientが共有するとき、frameが利用中かどうかと、available poolから外れる瞬間を
1つのcounterでどう表すか。

### 概念

Bufferはmutexで保護したnon-negative pin countを持つ。private `pin`はcountを1増やし、増加前が0だった
場合だけ`true`を返す。

```text
0 --pin(true)--> 1 --pin(false)--> 2 --pin(false)--> ...
```

`IsPinned`はcount > 0、`PinCount`は現在値を返す。availability accountingを担当する後続BufferManagerは
`pin`のbooleanを使い、同じframeの2回目以降のpinでavailable countを重複して減らさない。

### 公開API

| API | 責務 |
| --- | --- |
| `Buffer.IsPinned()` | pin countが1以上か返す |
| `Buffer.PinCount()` | mutexで保護したcurrent countを返す |
| private `Buffer.pin()` | countを増やし、0→1遷移だけを返す |

### Test-firstの記録

#### Red

fresh count、1回目と2回目のtransition、32 goroutinesのconcurrent pinを先にtestした。実装前は
`IsPinned`、`PinCount`、`pin`が存在せずbuildに失敗した。

```text
frame.IsPinned undefined
frame.PinCount undefined
frame.pin undefined
FAIL github.com/Rwatana/simledb_in_go/internal/buffer [build failed]
```

#### Green

Bufferへmutexとpin countを追加し、read/incrementを同じlockで保護した。

```text
ok github.com/Rwatana/simledb_in_go/internal/buffer
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestPinIncrementsCountAndReportsOnlyTheFirstTransition` | fresh zero、0→1 true、1→2 false |
| `TestPinIsConcurrencySafe` | 32 concurrent increments、exactly one availability transition |

## Javaソースリーディング

対象は`simpledb.buffer.Buffer.pin`と`isPinned`、`BufferMgr.tryToPin`のavailability分岐である。

| Java | Go | 判断 |
| --- | --- | --- |
| `pins++` | mutex内の`pins++` | 将来のconcurrent manager accessにもraceを残さない |
| `pins > 0` | `IsPinned()` | 同じpinned predicateを維持する |
| managerがincrement前に`isPinned` | `pin()`がtransition boolを返す | check-then-actを1 critical sectionにする |
| count getterなし | `PinCount()` | invariantをtestし診断できるread-only query |

## Clean Architecture上の判断

pin countはframe自身のlifecycle stateなのでBufferが所有する。pool全体のavailable countとframe選択は
後続BufferManagerへ残し、今回のPRは1 frame内のatomic transitionだけを扱う。

## 意図的に含めなかったもの

- unpin/decrementとunderflow error
- BufferManagerのavailable count
- dirty transaction/LSN metadata
- WAL付きflush
- block assignmentとreplacement

## Feature 10: Decrement buffer pin count

編集可能な図は[Draw.io版](../diagrams/ch04-buffer-unpin.drawio)、ブラウザで表示できる図は
[SVG版](../diagrams/ch04-buffer-unpin.svg)を参照する。図はpin count 2から1、1から0への遷移と、
0からの追加unpinをstate不変のerrorとして拒否する契約を示す。

### 問い

複数clientが共有するframeを解放するとき、まだ利用中の状態とavailableへ戻る瞬間を区別し、誤った
余分なunpinでcountが負になることをどう防ぐか。

### 概念

private `unpin`はBuffer mutex内でcountを検査する。count > 0なら1減らし、結果が0の場合だけ
`becameAvailable = true`を返す。すでに0ならerrorを返し、countを変更しない。

```text
2 --unpin(false)--> 1 --unpin(true)--> 0
0 --unpin(error)--> 0
```

後続BufferManagerはtransition booleanを使い、1→0のときだけavailable countを増やしてwaiterへ通知できる。

### 公開API

public APIは増やさない。private `Buffer.unpin() (becameAvailable bool, err error)`を追加し、既存の
`IsPinned`と`PinCount`で結果を観測する。

### Test-firstの記録

#### Red

2→1、1→0、0でのreject、32 goroutinesのconcurrent decrementを先にtestした。実装前は`unpin`が
存在せずbuildに失敗した。

```text
frame.unpin undefined (type *Buffer has no field or method unpin)
FAIL github.com/Rwatana/simledb_in_go/internal/buffer [build failed]
```

#### Green

pinと同じmutexを使うdecrement、transition判定、underflow errorを追加した。

```text
ok github.com/Rwatana/simledb_in_go/internal/buffer
```

### Test cases

| Test | 説明する振る舞い |
| --- | --- |
| `TestUnpinDecrementsCountAndReportsOnlyTheAvailableTransition` | 2→1 false、1→0 true、underflow reject |
| `TestUnpinIsConcurrencySafe` | 32 concurrent decrements、exactly one 1→0 transition、final zero |

## Javaソースリーディング

対象は`simpledb.buffer.Buffer.unpin`と`BufferMgr.unpin`である。

| Java | Go | 判断 |
| --- | --- | --- |
| `pins--` | zero検査後の`pins--` | negative countをsilentに作らない |
| managerが後から`isPinned` | `unpin`がtransition boolを返す | decrementと1→0判定をatomicにする |
| underflow規定なし | explicit error | duplicate releaseを呼び出し側bugとして報告する |
| manager method全体をsynchronized | frame mutex | 1 frameのstate transitionを局所化する |

## Clean Architecture上の判断

pinとunpinのcounter invariantはBuffer自身が守る。available poolの集計とcondition notificationはまだ
BufferManagerへ置かず、今回のPRを1 frameのrelease transitionだけに限定する。

## 意図的に含めなかったもの

- BufferManagerのavailable count更新
- waiting goroutine notification
- dirty transaction/LSN metadata
- WAL付きflush
- block assignmentとreplacement

## 次の機能

次はBuffer frameを変更したtransaction IDだけを記録する。LSN metadata、WAL付きflush、block assignment、
BufferManager poolは、それぞれさらに独立したIssue、commit、stacked PRへ分ける。

## 参照元

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Chapter 4
- SimpleDB 3.4 Java source: `simpledb.log.LogMgr`
