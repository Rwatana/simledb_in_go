# Contributing

## 変更の進め方

1. 章または振る舞いを1つ選び、Issueを作る
2. titleに変更対象のprefixを付け、受け入れ条件と対象外を書く
3. `feature/#<issue-number>_<snake_case_description>`形式でbranchを作る
4. 実装PRでは失敗するテストを先に追加する
5. 最小実装、refactor、章ノート更新の順に進める
6. Pull Request titleに同じprefixを付け、本文にIssue Numberを書く
7. Pull Request本文に`Closes #<issue>`を書く

## Title prefix

IssueとPull Requestのtitleは`[Prefix] Summary`形式にする。許可するprefixの原本は
`hack/prefix.yml`とし、CIでも同じファイルを参照する。

| Prefix | 対象 | 例 |
| --- | --- | --- |
| `[Docs]` | 章まとめ、ソースリーディング、Draw.io、文書 | `[Docs] Summarize Chapter 1` |
| `[Go]` | Goの実装、テスト、refactor | `[Go] Implement BlockID` |
| `[CI/CD]` | GitHub Actions、template、開発自動化 | `[CI/CD] Validate Pull Request policy` |

1つの変更が複数に関係する場合は、主目的のprefixを1つだけ選ぶ。

## Branch名

Twitter-Cloneの標準形式である
`feature/#<issue-number>_<snake_case_description>`を使う。

- issue number: 対応するGitHub Issue番号
- description: 内容を表す小文字のsnake_case
- `/`、`#`、Issue番号、`_`を省略しない

例: `feature/#12_implement_block_id`、`feature/#1_chapter_1_database_systems`

branchのIssue番号、PR本文の`## Issue Number`、`Closes #<issue-number>`はすべて
同じ番号にする。PR番号はGitHubが自動採番する別の番号なので、手動では設定しない。

## Pull Request本文

Twitter-Cloneのtemplateから、次の適用可能な項目を引き継ぐ。

- `Issue Number`: branchと同じIssue番号
- `Implementation Summary`: 変更内容と理由
- `Scope of Impact`: 影響範囲と対象外
- `Particular points to check`: reviewで重点的に見る点
- `Learning Notes`: Java版との対応、Goの設計判断、意図的な差異
- `Test`: 実行したtestとdocs/diagram検証

`Schedule`は自己学習の各PRで必須にする意味が薄いため除外する。期限が必要な作業だけ
Issueのmilestoneまたは本文へ記録する。

## 1 Pull Request = 1 Commit

Twitter-Cloneで採用した運用を踏襲し、PR branchは1 commitに保つ。修正時はcommitを
追加せず、既存commitをamendしてforce-with-leaseで更新する。mainへはsquash mergeする。

```sh
git add <files>
git commit --amend --no-edit
git push --force-with-lease
```

履歴を書き換えるのは作業branchだけに限定し、共有branchのmainでは行わない。

## Pull Requestの大きさ

1つのPRで「学習資料」「値オブジェクト」「page encoding」「file I/O」のように
主題を1つに絞る。レビューに別の概念が必要になったら、後続Issueへ分ける。

## Goコード

- `gofmt`を適用する
- `go test ./...`と`go vet ./...`を通す
- exported APIにはGoのdoc commentを書く
- package名は小文字の短い単語にする
- interfaceは利用側に置き、必要なmethodだけを含める
- errorには失敗した操作と対象が分かる文脈を加える

## 学習ノート

章ノートには次を含める。

- 章の中心となる問い
- DB概念を自分の言葉で説明した要約
- Java版で読むpackage、class、test
- Goへ移すときの設計判断と、Java版との差異
- テスト観点
- 次の最小Issue候補

書籍本文やソースコードを大量に転載せず、要約と参照箇所を記録する。

## Draw.io

- `.drawio`を編集可能な原本としてcommitする
- 図だけで新しい仕様を定義せず、対応するMarkdownにも説明を書く
- PR前にdraw.ioまたはdiagrams.netで開けることを確認する

## CI

Pull Requestでは次を自動検証する。

- IssueとPR titleのprefixが`hack/prefix.yml`に存在する
- branch名が規約どおりである
- branch、PR本文のIssue Number、`Closes`の番号が一致する
- PRのcommit数が1である
- Markdownの相対リンクが存在する
- Draw.ioファイルが正しいXMLである
- `go.mod`追加後は`go test ./...`と`go vet ./...`が成功する

## Twitter-Cloneから除外した規則

- `[TS]`、`[iOS]`、`[BFF]`、`[Infra]`、`[Go/TS]`: このrepositoryに対象componentがない
- 必須`Schedule`: 自己学習の進行を不要な締切で拘束しない

対象componentや運用が増えた場合は、必要になった時点で別Issueとして追加する。
