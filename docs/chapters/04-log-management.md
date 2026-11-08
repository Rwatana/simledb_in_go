# Chapter 4: Memory and Log Management

## この章の進め方

Chapter 4ではChapter 3のfixed-size file layerを使い、recovery logとbuffer管理を段階的に実装する。
logの振る舞いを小さく分け、初期化、1 block内append、flush、block rollover、reverse iterationを
それぞれ独立したIssue、test、1 commit、stacked Pull Requestとして扱う。

| Feature | 学ぶ概念 | 状態 |
| --- | --- | --- |
| log block initialization | tail pageとboundary | 実装済み |
| record append in one block | right-to-left encodingとLSN | 未着手 |
| explicit flush | durability boundary | 未着手 |
| block rollover | full tailから次blockへの遷移 | 未着手 |
| reverse iterator | newest-to-oldest traversal | 未着手 |

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

## 次の機能

次は現在のtail blockに収まる1 recordだけをright-to-leftでappendし、boundaryを更新してLSNを返す。
disk flushと容量不足時のrolloverは、それぞれ別Issueと別PRへ分ける。

## 参照元

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Chapter 4
- SimpleDB 3.4 Java source: `simpledb.log.LogMgr`
