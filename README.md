# aiwolf-nlp-server

人狼知能コンテスト（自然言語部門） のゲームサーバです。

サンプルエージェントについては、[aiwolfdial/aiwolf-nlp-agent](https://github.com/aiwolfdial/aiwolf-nlp-agent) を参考にしてください。

> [!IMPORTANT]
> 次回大会では以下の変更があります。詳細については、[aiwolfdial.github.io/aiwolf-nlp](https://aiwolfdial.github.io/aiwolf-nlp)を参照してください。
>
> - [発言の文字数制限](./doc/logic.md#発言の文字数制限について)
> - [13人ゲーム](./doc/logic.md#13人ゲーム)
> - [プレイヤー名とプロフィール](./doc/protocol.md#info)

## ドキュメント

- [プロトコルの実装について](./doc/protocol.md)
- [ゲームロジックの実装について](./doc/logic.md)
- [設定ファイルについて](./doc/config.md)

## 実行方法

デフォルトのサーバアドレスは `ws://127.0.0.1:8080/ws` です。エージェントプログラムの接続先には、このアドレスを指定してください。\
同じチーム名のエージェント同士のみをマッチングさせる自己対戦モードは、デフォルトで有効になっています。そのため、異なるチーム名のエージェント同士をマッチングさせる場合は、設定ファイルを変更してください。\
設定ファイルの変更方法については、[設定ファイルについて](./doc/config.md)を参照してください。

### Linux

```bash
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/aiwolf-nlp-server-linux-amd64
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/default_5.yml
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/default_13.yml
# ダイナミックプロフィールを使用する場合は、以下のコマンドを実行してください。
# curl -Lo .env https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/example.env
chmod u+x ./aiwolf-nlp-server-linux-amd64
./aiwolf-nlp-server-linux-amd64 -c ./default_5.yml # 5人ゲームの場合
# ./aiwolf-nlp-server-linux-amd64 -c ./default_13.yml # 13人ゲームの場合
```

### Windows

```bash
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/aiwolf-nlp-server-windows-amd64.exe
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/default_5.yml
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/default_13.yml
# ダイナミックプロフィールを使用する場合は、以下のコマンドを実行してください。
# curl -Lo .env https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/example.env
.\aiwolf-nlp-server-windows-amd64.exe -c .\default_5.yml # 5人ゲームの場合
# .\aiwolf-nlp-server-windows-amd64.exe -c .\default_13.yml # 13人ゲームの場合
```

### macOS (Intel)

> [!NOTE]
> 開発元が不明なアプリケーションとしてブロックされる場合があります。\
> 下記サイトを参考に、実行許可を与えてください。  
> <https://support.apple.com/ja-jp/guide/mac-help/mh40616/mac>

```bash
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/aiwolf-nlp-server-darwin-amd64
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/default_5.yml
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/default_13.yml
# ダイナミックプロフィールを使用する場合は、以下のコマンドを実行してください。
# curl -Lo .env https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/example.env
chmod u+x ./aiwolf-nlp-server-darwin-amd64
./aiwolf-nlp-server-darwin-amd64 -c ./default_5.yml # 5人ゲームの場合
# ./aiwolf-nlp-server-darwin-amd64 -c ./default_13.yml # 13人ゲームの場合
```

### macOS (Apple Silicon)

> [!NOTE]
> 開発元が不明なアプリケーションとしてブロックされる場合があります。\
> 下記サイトを参考に、実行許可を与えてください。  
> <https://support.apple.com/ja-jp/guide/mac-help/mh40616/mac>

```bash
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/aiwolf-nlp-server-darwin-arm64
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/default_5.yml
curl -LO https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/default_13.yml
# ダイナミックプロフィールを使用する場合は、以下のコマンドを実行してください。
# curl -Lo .env https://github.com/iggy157/aiwolf-nlp-server-unlimited/releases/latest/download/example.env
chmod u+x ./aiwolf-nlp-server-darwin-arm64
./aiwolf-nlp-server-darwin-arm64 -c ./default_5.yml # 5人ゲームの場合
# ./aiwolf-nlp-server-darwin-arm64 -c ./default_13.yml # 13人ゲームの場合
```
