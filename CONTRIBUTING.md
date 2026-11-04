# Contributing

## 変更の進め方

1. 章または振る舞いを1つ選び、Issueを作る
2. 受け入れ条件と対象外を書く
3. `type/<issue-number>-<short-description>`形式でbranchを作る
4. 実装PRでは失敗するテストを先に追加する
5. 最小実装、refactor、章ノート更新の順に進める
6. Pull Request本文に`Closes #<issue>`を書く

## Branch名

`type/<issue-number>-<short-description>`を使う。

- type: `docs`, `feat`, `fix`, `refactor`, `test`, `chore`
- issue number: 対応するGitHub Issue番号
- short description: 小文字のkebab-case

例: `feat/12-block-id`、`docs/1-chapter-01-database-systems`

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

- branch名が規約どおりである
- PRのcommit数が1である
- Markdownの相対リンクが存在する
- Draw.ioファイルが正しいXMLである
- `go.mod`追加後は`go test ./...`と`go vet ./...`が成功する
