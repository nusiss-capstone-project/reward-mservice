# reward-mservice

Go 微服务（`common` / `client` / `server` 三模块）。

## 模块路径

```
github.com/nusiss-capstone-project/reward-mservice/{common|client|server}
```

## 本地开发

```bash
export MYSQL_DSN='user:pass@tcp(127.0.0.1:3306)/reward_db?charset=utf8mb4&parseTime=True&loc=Local'
cd server && go run main.go
```

### 临时 Kafka（本地联调）

审批通过后会向 `reward.finance_doc.approved` 发消息，本地可用 Docker 起一个单节点 Kafka（KRaft，无需 ZooKeeper）。

**Docker 拉取慢 / 超时**

`docker-compose.kafka.yml` 已默认使用 DaoCloud 镜像代理：

```yaml
image: docker.m.daocloud.io/apache/kafka:3.9.0
```

也可在 Docker Desktop 全局配置 registry mirror（Settings → Docker Engine），重启 Docker 后生效：

```json
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://docker.1ms.run"
  ]
}
```

若走 HTTP 代理，在 Docker Desktop → Settings → Resources → Proxies 填写代理地址。

**启动：**

```bash
docker compose -f docker-compose.kafka.yml up -d
```

**停止并删除：**

```bash
docker compose -f docker-compose.kafka.yml down
```

**确认 broker 已就绪：**

```bash
docker logs reward-kafka-local
```

然后在 `server/resources/config.yml` 中启用 Kafka（默认 broker 为 `localhost:9092`）：

```yaml
kafka:
  enabled: true
  brokers:
    - localhost:9092
  group_id: reward-mservice
  client_id: reward-mservice
  topics:
    - reward.finance_doc.approved
```

Topic 会在首次 produce 时自动创建（Kafka 默认开启 `auto.create.topics.enable`）。若需手动创建：

```bash
docker exec reward-kafka-local /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --create --topic reward.finance_doc.approved --partitions 1 --replication-factor 1
```

本地不想跑 Kafka 时，将 `kafka.enabled` 设为 `false`；producer 会使用 no-op 实现，审批接口仍可正常改状态，但不会初始化 `project_budget`。

## API

| 类型 | 地址 |
|------|------|
| 健康检查 | `GET /reward-ms/v1/ping` |
| Swagger | `/reward-ms/v1/swagger/index.html` |
| gRPC | `RewardService`（端口 `5001`） |
| HTTP | 端口 `8080` |

## 配置

- Go：`1.25.10`
- MySQL 库名：`reward_ms_db`
- Proto：`common/proto/reward.proto`（package `rewardpb`）
