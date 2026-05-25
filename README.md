<p align="center">
  <img src="./docs/logo.svg" alt="dokan logo" width="460" />
</p>

<p align="center">
  AWS Systems Manager Session Manager 経由の接続を簡単にする、RDS/DocumentDB 向けポートフォワード CLI
</p>

<p align="center">
  <a href="https://github.com/tomozo6/dokan/releases"><img src="https://img.shields.io/github/v/release/tomozo6/dokan?label=release" alt="release"></a>
  <a href="https://github.com/tomozo6/dokan/blob/main/LICENSE"><img src="https://img.shields.io/github/license/tomozo6/dokan" alt="license"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go" alt="go"></a>
</p>

## dokanとは

`dokan` は、踏み台 EC2 を経由した RDS / DocumentDB への SSM ポートフォワードを対話形式で実行する CLI です。

- AWS マネジメントコンソールを開かずに接続準備できる
- 踏み台 EC2 と接続先 DB クラスタを対話で選択できる
- `writer` / `reader` の接続先をフラグで切り替えられる

## クイックスタート

```bash
brew install tomozo6/tap/dokan
dokan
```

実行後、対話形式で以下を選択します。

1. 踏み台 EC2
2. 接続先 DB クラスタ

デフォルトでは `reader` エンドポイントへ接続し、ローカルポートは DB ポートと同じ番号になります。

## インストール

### Homebrew (macOS / Linux)

```bash
brew install tomozo6/tap/dokan
```

### Scoop (Windows PowerShell)

```powershell
scoop bucket add tomozo6 https://github.com/tomozo6/scoop-bucket
scoop install dokan
```

### バイナリ

[Releases](https://github.com/tomozo6/dokan/releases) からダウンロードできます。

## 使い方

詳細は `dokan [command] --help` を参照してください。

### 基本コマンド

```bash
dokan
```

踏み台 EC2 と DB クラスタを選択してポートフォワードします。

### 主要オプション

```bash
# writer インスタンスへ接続
dokan --writer

# AWS プロファイルを指定
dokan --profile stg

# ローカルポートを指定
dokan --port 3333
```

### EC2へログイン

```bash
dokan ec2login
```

対話形式で EC2 を選択し、SSM Session Manager でログインします。
(`bash` でログインするため、接続先に `bash` が必要です)

## AWS 認証情報の参照順

`dokan` は次の優先順で AWS プロファイルを解決します。

1. `--profile` フラグ
2. `AWS_PROFILE` 環境変数
3. `AWS_DEFAULT_PROFILE` 環境変数
4. AWS SDK のデフォルト認証チェーン

PowerShell 例:

```powershell
$env:AWS_DEFAULT_PROFILE = "your-profile"
dokan
```

## 前提条件

### Session Manager Plugin

`dokan` の利用には、AWS の Session Manager Plugin が必要です。

[AWS CLI 用 Session Manager プラグインのインストール手順](https://docs.aws.amazon.com/ja_jp/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)

### IAM 権限

実行ユーザーには、少なくとも以下が必要です。

- SSM セッション開始関連権限
- EC2 情報参照権限
- RDS / DocumentDB クラスタ情報参照権限

詳細ポリシー例は整備中です。
