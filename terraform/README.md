# terraform

dev環境のAPIを実際に稼働させるVPC/ALB/ECS Fargate/DocumentDBをTerraformで構築する。

g5自身は認証情報(Cognitoユーザープール)を持たない。サインアップ/サインインは
[g4](https://github.com/k07g/g4)が所有するCognitoユーザープールが担当し、g5は
そのプールが発行したアクセストークンを検証するだけなので、Cognito関連の
リソースはここでは作成しない(詳細は「dev環境のAPIインフラ」の節を参照)。

## 構成

```
terraform/
  bootstrap/                 # state用S3、GitHub Actions用OIDC IAMロール、ECRリポジトリを作る(初回のみ手動実行)
  environments/dev/          # dev環境のエントリーポイント。mainマージ時にCIが自動applyする
```

g4と異なり、g5には`modules/`や`sandbox`環境が無い。g4の`sandbox`はCognito
(ほぼ無料)を単体で試すためのものだったが、g5にはそれに相当する「安価に
単体で試せるリソース」が無く(DocumentDBはVPC必須かつ最小構成でも常時課金
される)、dev環境が唯一のデプロイ対象になっている。

| 環境 | state | apply方法 | 実データ |
| --- | --- | --- | --- |
| dev | S3 backend(`bootstrap`で作成) | **mainブランチへのマージ時にGitHub Actionsが自動apply** | VPC/ALB/ECS Fargate/DocumentDB(APIが実際に稼働) |

## 前提

- Terraform >= 1.10(dev環境のS3ネイティブロックに必要)
- AWS認証情報(環境変数 / `~/.aws/credentials` など)が設定済みであること
- [g4](https://github.com/k07g/g4)のdev環境Cognitoユーザープールが既にデプロイ済みであること
  (そのARNをg5のdev環境に変数として渡す必要がある。下記参照)

## 0. 初回セットアップ(bootstrap、手動・一度だけ)

dev環境をCIから自動applyするには、事前にTerraform state用のS3バケットと、
GitHub ActionsがOIDCでAssumeRoleするためのIAMロールが必要。stateのロックは
DynamoDBではなくS3ネイティブロック(`use_lockfile`、Terraform 1.10+)を使うため
別途ロック用テーブルは不要。これは`terraform/bootstrap`で構築するが、循環依存
(stateを保存する場所自体をTerraformで作る)を避けるためローカルstateのまま、
AWS管理者権限を持つ人がローカルから一度だけ実行する。

```sh
cd terraform/bootstrap
terraform init
terraform apply \
  -var="state_bucket_name=<グローバルに一意なバケット名>"
```

- `state_bucket_name` は必須(S3バケット名はAWS全体で一意である必要がある)
- AWSアカウントに既にGitHub Actions用のOIDCプロバイダが存在する場合
  (例えば同じAWSアカウントで[g4](https://github.com/k07g/g4)のbootstrapを
  先に実行済みの場合。1アカウントにつきプロバイダは1つまでしか作成できない)は
  `-var="create_github_oidc_provider=false" -var="existing_github_oidc_provider_arn=<既存のARN>"`
  を追加する

apply後、以下をGitHubリポジトリの **Settings > Secrets and variables > Actions > Variables**
に登録する(値はいずれも機密ではないため Secrets ではなく Variables でよい)。

| GitHub Actions variable | 値 |
| --- | --- |
| `AWS_DEV_TERRAFORM_ROLE_ARN` | `terraform output github_actions_role_arn` |
| `TF_STATE_BUCKET` | `terraform output state_bucket_name` |
| `AWS_ECR_PUSH_ROLE_ARN` | `terraform output github_actions_ecr_push_role_arn` |
| `ECR_REPOSITORY` | `terraform output ecr_repository_name`(既定値 `g5`) |
| `AWS_REGION` | 任意(未設定時は `ap-northeast-1`) |

加えて、g4のCognitoユーザープールのARNも変数として必要になる(下記「1. dev環境」参照)。

| GitHub Actions variable | 値 |
| --- | --- |
| `COGNITO_USER_POOL_ARN` | g4のdev環境Cognitoユーザープールのarn(g4のterraform outputには現状含まれないため、AWSコンソールまたは`aws cognito-idp describe-user-pool`で取得する) |

`terraform/bootstrap` の `terraform.tfstate` はこのbootstrap自体の管理に必要なので、
誤って削除しないこと(このディレクトリはめったに変更しない想定)。

## 1. dev環境: mainマージで自動apply

[.github/workflows/terraform-dev-apply.yml](../.github/workflows/terraform-dev-apply.yml) が、
`main` ブランチへのpush(マージ)のうち `terraform/environments/dev/**` に変更が
あった場合に、GitHub ActionsのOIDCでAWSにAssumeRoleし
`terraform init && plan && apply` を自動実行する。`cognito_user_pool_arn` 変数は
`TF_VAR_cognito_user_pool_arn` 環境変数(GitHub Actions変数 `COGNITO_USER_POOL_ARN`
から設定)経由で渡している。

- 手動での再実行は Actions タブから `workflow_dispatch` で可能
- apply前に人手のレビューを挟みたい場合は、リポジトリの
  **Settings > Environments > dev** で Required reviewers を設定すると、
  ワークフロー変更なしに承認ゲートを追加できる
- 同時実行はconcurrency groupで直列化され、state競合を防いでいる

ローカルから同じdev stateを操作したい場合は、`backend.hcl.example` を参考に
`backend.hcl` を作成してから初期化する(`backend.hcl` は秘密情報ではないが
バケット名等が環境ごとに異なるため `.gitignore` 対象)。

```sh
cd terraform/environments/dev
cp backend.hcl.example backend.hcl   # 値をbootstrap出力に合わせて編集
terraform init -backend-config=backend.hcl
terraform plan -var="cognito_user_pool_arn=<g4のCognitoユーザープールarn>"
```

## 2. アプリ: mainマージで自動デプロイ(ECR push → ECS更新)

[.github/workflows/deploy-dev.yml](../.github/workflows/deploy-dev.yml) が、
`main` ブランチへのpushのうちアプリのソース(`cmd/**`, `internal/**`, `go.mod`,
`go.sum`, `Dockerfile`)に変更があった場合に、以下を自動で行う。

1. イメージをビルドしECR(bootstrapで作成)に `<commit SHA>` タグと `latest` タグでpush
2. dev環境のECSタスク定義(`g5-dev`)の最新リビジョンを取得し、イメージだけを
   新しいSHAタグに差し替えて新しいリビジョンを登録
3. ECSサービス(`g5-dev`)をその新しいリビジョンに更新し、安定するまで待機

認証はdevのTerraform applyと同様GitHub ActionsのOIDCを使うが、`environment:`
は指定せず main ブランチへのpushを直接信頼するロール(`AWS_ECR_PUSH_ROLE_ARN`。
ECR pushとECSデプロイの両方の権限を持つ)を使う。

ECSサービス・タスク定義そのもの(CPU/メモリ、ロール、ログ設定、Secrets参照
など)は `terraform-dev-apply.yml` が管理するが、`aws_ecs_service` の
`task_definition` は `lifecycle.ignore_changes` で無視しているため、
このワークフローが登録する新しいリビジョンをTerraform applyが巻き戻すことはない。

手動での再実行は Actions タブから `workflow_dispatch` で可能。

g4と異なり、g5はCognitoクライアント情報を持たないため`.env`に設定すべき
Terraform出力は基本的に無い(`MONGODB_URI`はECSタスクにSecrets Manager経由で
直接注入され、ローカル開発では代わりに`docker compose up -d mongo`を使う。
[トップレベルのREADME](../README.md)を参照)。

## dev環境のAPIインフラ

- VPC(既定 `10.30.0.0/16`)+ 2つのパブリックサブネット。NAT Gatewayは使わず、
  ALB・ECSタスク・DocumentDBをすべてパブリックサブネットに配置して固定費を抑える
  (ECSタスクにパブリックIPを付与してECR/Cognitoに直接到達)
- ALB: AWS提供ドメインでHTTP(80番)公開。独自ドメイン/HTTPSは未設定
- ECS Fargate: 最小構成(0.25 vCPU / 512MiB)、`desired_count = 1`
- Amazon DocumentDB(MongoDB互換、既定 `db.t3.medium`): クラスタは常にVPC内にのみ
  存在し(RDSの`publicly_accessible`に相当する設定自体が無い)、ECSサービスの
  セキュリティグループからの接続のみ許可。RDSの`db.t4g.micro`のようなバースト
  可能な最小構成が無く、`db.t3.medium`が実質的な最小インスタンスクラスのため、
  g4のRDSより固定費が高くなる点に注意
- DocumentDBはデフォルトでTLSが必須。Dockerイメージのビルド時にAmazonの
  CA証明書バンドル(`global-bundle.pem`)を`/etc/ssl/certs/rds-global-bundle.pem`に
  焼き込んでおり([../Dockerfile](../Dockerfile))、`MONGODB_URI`シークレットの
  `tlsCAFile`パラメータでそれを指す。またDocumentDBは常にレプリカセット
  (`rs0`固定)として振る舞い、retryable writesをサポートしないため
  `replicaSet=rs0&retryWrites=false`も接続文字列に含めている
  ([docdb.tf](environments/dev/docdb.tf))
- DBの接続文字列は平文でタスク定義に埋め込まず、Secrets Manager経由で
  コンテナに注入する(`MONGODB_URI`)
- Cognitoのユーザープールは持たないが、ECSタスクロールに
  `cognito-idp:GetUser`のみを、g4のユーザープールARN(`cognito_user_pool_arn`
  変数)に限定して付与している。それ以外のCognito操作(サインアップ/
  サインイン等)はg4側の責務

apply後、APIのURLは以下で確認できる。

```sh
cd terraform/environments/dev
terraform output api_url
```

初回applyの時点ではECRにまだイメージが無く、ECSタスクは起動に失敗し続ける
(想定内)。[.github/workflows/deploy-dev.yml](../.github/workflows/deploy-dev.yml)
が一度実行されてイメージがpushされると正常化する。

本番相当の環境を作る場合は、`environments/` 配下に `stg` / `prod` などを追加する
こと。CIから自動applyする場合は`bootstrap`のIAMロールの信頼ブランチ・権限範囲を
環境ごとに分けることを検討する。

## 破棄

```sh
cd terraform/environments/dev
terraform destroy -var="cognito_user_pool_arn=<g4のCognitoユーザープールarn>"
```

state用のS3バケットやOIDC IAMロール自体を破棄する場合は
`terraform/bootstrap` で `terraform destroy` するが、他環境が同じバケットを
参照していないことを確認してから実行すること。
