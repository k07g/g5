# terraform

dev環境のAPIを実際に稼働させるALB/ECS FargateをTerraformで構築する。

g5自身は認証情報(Cognitoユーザープール)を持たない。サインアップ/サインインは
[g4](https://github.com/k07g/g4)が所有するCognitoユーザープールが担当し、g5は
そのプールが発行したアクセストークンを検証するだけなので、Cognito関連の
リソースはここでは作成しない。

**DBについても同様に、g5専用のインスタンスは一旦作らず、g4が運用している
RDS PostgreSQLインスタンスを暫定的に共用する**(元々はMongoDB/Amazon
DocumentDBの採用を検討していたが、コスト面([#7](../../issues/7)参照)から
一旦見送り、独自DBの構築自体を先送りにしている)。g5のECSタスクはg4の
既存VPC・パブリックサブネットにそのままデプロイし(g5専用のVPCは作らない)、
g4のRDSセキュリティグループに対してg5のECSサービスからのアクセスを許可する
ingressルールをg5側のTerraformから追加する形で共用する。**この構成はあくまで
暫定措置であり、将来的にはg5専用のデータストア(当初の計画通りMongoDB)に
分離する予定。詳細と経緯は[#7](../../issues/7)を参照。**

## 構成

```
terraform/
  bootstrap/                 # state用S3、GitHub Actions用OIDC IAMロール、ECRリポジトリを作る(初回のみ手動実行)
  environments/dev/          # dev環境のエントリーポイント。mainマージ時にCIが自動applyする
```

g4と異なり、g5には`modules/`や`sandbox`環境が無い。CognitoもDBもg4を間借り
しており、g5自身が単体で持つリソースがほぼ無いため。

| 環境 | state | apply方法 | 実データ |
| --- | --- | --- | --- |
| dev | S3 backend(`bootstrap`で作成) | **mainブランチへのマージ時にGitHub Actionsが自動apply** | ALB/ECS Fargate(APIが実際に稼働。VPC/DBはg4のものを間借り) |

## 前提

- Terraform >= 1.10(dev環境のS3ネイティブロックに必要)
- AWS認証情報(環境変数 / `~/.aws/credentials` など)が設定済みであること
- [g4](https://github.com/k07g/g4)のdev環境(Cognitoユーザープール・VPC・RDS)が
  既にデプロイ済みであること(それらの値をg5のdev環境に変数として渡す必要が
  ある。下記参照)

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

加えて、g4側の既存リソースを間借りするための変数も必要になる(g4のterraform
outputには現状含まれないため、AWSコンソールまたはAWS CLIで取得する)。

| GitHub Actions variable | 値 | 取得方法の例 |
| --- | --- | --- |
| `COGNITO_USER_POOL_ARN` | g4のdev環境Cognitoユーザープールのarn | `aws cognito-idp describe-user-pool --user-pool-id <g4のuser_pool_id>` |
| `G4_VPC_ID` | g4のdev環境VPC ID | `aws ec2 describe-vpcs --filters Name=tag:Name,Values=g4-dev` |
| `G4_PUBLIC_SUBNET_IDS` | g4のdev環境パブリックサブネットID一覧。**JSON配列の文字列**で指定する(例: `["subnet-0123...","subnet-0456..."]`) | `aws ec2 describe-subnets --filters Name=tag:Name,Values=g4-dev-public-*` |
| `G4_RDS_SECURITY_GROUP_ID` | g4のdev環境RDSインスタンスに付与されているセキュリティグループID | `aws rds describe-db-instances --db-instance-identifier g4-dev` の `VpcSecurityGroups` |

`terraform/bootstrap` の `terraform.tfstate` はこのbootstrap自体の管理に必要なので、
誤って削除しないこと(このディレクトリはめったに変更しない想定)。

## 1. g4のRDSにg5専用のデータベース/ロールを手動で作成する(初回のみ)

g5のTerraformはCI環境からg4のRDS(`publicly_accessible = false`)へ
ネットワーク到達できないため、データベース/ロールの作成とアプリ接続文字列の
登録はTerraformの外で手動で行う。

1. g4のRDSに(踏み台やVPC内の一時的な手段経由で)接続し、g5専用のデータベースと
   ロールを作成する。g4の`users`テーブルとは完全に別のデータベースにして
   データを分離する。

   ```sql
   CREATE DATABASE g5;
   CREATE ROLE g5 WITH LOGIN PASSWORD '<強力なランダムパスワード>';
   GRANT ALL PRIVILEGES ON DATABASE g5 TO g5;
   ```

2. `internal/db/migrations/0001_create_career_sheets_table.up.sql` を
   このg5データベースに適用する(アプリ起動時にも自動実行されるため、
   初回は省略しても後で自動適用される)。

3. `terraform apply`(後述)でg5側の`database_url_secret_arn`出力先の
   シークレットが作成された後、接続文字列を投入する。

   ```sh
   aws secretsmanager put-secret-value \
     --secret-id "$(cd terraform/environments/dev && terraform output -raw database_url_secret_arn)" \
     --secret-string "postgres://g5:<パスワード>@<g4のRDSエンドポイント>:5432/g5?sslmode=require"
   ```

初回applyの時点ではこのシークレットに値が入っていないため、ECSタスクは
起動に失敗し続ける(想定内)。上記の手動投入後、ECSサービスが新しい
シークレット値を拾うまで再デプロイ(`aws ecs update-service --force-new-deployment`
または[.github/workflows/deploy-dev.yml](../.github/workflows/deploy-dev.yml)の
再実行)が必要な場合がある。

## 2. dev環境: mainマージで自動apply

[.github/workflows/terraform-dev-apply.yml](../.github/workflows/terraform-dev-apply.yml) が、
`main` ブランチへのpush(マージ)のうち `terraform/environments/dev/**` に変更が
あった場合に、GitHub ActionsのOIDCでAWSにAssumeRoleし
`terraform init && plan && apply` を自動実行する。g4関連の変数は
`TF_VAR_*` 環境変数(上記のGitHub Actions変数から設定)経由で渡している。

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
terraform plan \
  -var="cognito_user_pool_arn=<g4のCognitoユーザープールarn>" \
  -var="g4_vpc_id=<g4のVPC ID>" \
  -var='g4_public_subnet_ids=["<subnet-id-1>","<subnet-id-2>"]' \
  -var="g4_rds_security_group_id=<g4のRDSセキュリティグループID>"
```

## 3. アプリ: mainマージで自動デプロイ(ECR push → ECS更新)

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

## dev環境のAPIインフラ

- VPC/パブリックサブネットはg4のdev環境のものをそのまま使う(g5専用VPCは
  作らない)。ALB・ECSタスクはg4のパブリックサブネットに配置し、ECSタスクには
  パブリックIPを付与してECR/Cognitoに直接到達させる
- ALB: AWS提供ドメインでHTTP(80番)公開。独自ドメイン/HTTPSは未設定
- ECS Fargate: 最小構成(0.25 vCPU / 512MiB)、`desired_count = 1`
- DB: g4のRDS PostgreSQLインスタンスを共用する。g5用のセキュリティグループ
  ルールをg4のRDSセキュリティグループに追加してアクセスを許可している
  (`aws_vpc_security_group_ingress_rule`は独立したルールリソースなので、
  g4側のセキュリティグループ本体やTerraform stateには一切触れない。
  [security_groups.tf](environments/dev/security_groups.tf)参照)
- DBの接続文字列は平文でタスク定義に埋め込まず、Secrets Manager経由で
  コンテナに注入する(`DATABASE_URL`)。ただしg4のRDSへの初回のデータベース/
  ロール作成とこのシークレットへの値投入は手動で行う必要がある
  (上記「1. g4のRDSにg5専用のデータベース/ロールを手動で作成する」参照)
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
が一度実行されてイメージがpushされると正常化する(DBシークレットへの
手動投入が済んでいることも前提)。

本番相当の環境を作る場合は、`environments/` 配下に `stg` / `prod` などを追加する
こと。CIから自動applyする場合は`bootstrap`のIAMロールの信頼ブランチ・権限範囲を
環境ごとに分けることを検討する。

## 破棄

```sh
cd terraform/environments/dev
terraform destroy \
  -var="cognito_user_pool_arn=<g4のCognitoユーザープールarn>" \
  -var="g4_vpc_id=<g4のVPC ID>" \
  -var='g4_public_subnet_ids=["<subnet-id-1>","<subnet-id-2>"]' \
  -var="g4_rds_security_group_id=<g4のRDSセキュリティグループID>"
```

g4のRDSに手動で作成した`g5`データベース/ロールはTerraformの管理外なので、
不要になった場合は別途手動で`DROP DATABASE g5; DROP ROLE g5;`すること。

state用のS3バケットやOIDC IAMロール自体を破棄する場合は
`terraform/bootstrap` で `terraform destroy` するが、他環境が同じバケットを
参照していないことを確認してから実行すること。
