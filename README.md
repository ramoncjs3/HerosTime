# oldbeggar-refactor

这是“老乞丐推送”的 Go 重构版骨架，目标是把官方、苹果、H5、暴走几套重复代码收敛成一个配置驱动的长期运行程序。

## 服务器部署

当前服务器上的新程序部署在：

```bash
/opt/bzyxt-oldbeggar/refactor
```

Docker 运行信息：

- Compose 项目名：`bzyxt-oldbeggar-refactor`
- 容器：`bzyxt-oldbeggar-refactor-app`
- 网络：`bzyxt-oldbeggar-refactor-net`
- 配置文件：`configs/config.local.yaml`
- 状态目录：`state/`

新程序不再使用 QQBot。生产配置里 `qq.api_url` 和 `qq.access_token` 保持为空，通过 WxPusher 推送。不要把新程序挂到老程序的 `bzyxt-oldbeggar-net`，也不要部署到 `/opt/bzyxt-oldbeggar/deploy`。

部署或重启：

```bash
cd /opt/bzyxt-oldbeggar/refactor
docker compose -f docker-compose.server.yml up -d --build app
```

检查状态：

```bash
docker compose -f docker-compose.server.yml ps
docker inspect bzyxt-oldbeggar-refactor-app --format 'networks={{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}'
docker logs -f --tail=120 bzyxt-oldbeggar-refactor-app
```

## 管理后台

程序内置一个 Vue 管理后台，默认关闭。开启时必须提供账号和密码：

```bash
cd /opt/bzyxt-oldbeggar/refactor
export OLDBEGGAR_ADMIN_ENABLED=1
export OLDBEGGAR_ADMIN_USERNAME=admin
export OLDBEGGAR_ADMIN_PASSWORD='换成强密码'
docker compose -f docker-compose.server.yml up -d --build app
```

默认端口映射为 `127.0.0.1:8088:8088`，适合用 SSH 隧道访问：

```bash
ssh -J root@8.148.181.9 -L 18088:127.0.0.1:8088 root@36.137.84.162
```

本地打开 `http://127.0.0.1:18088`。如果要直接公网开放，设置 `OLDBEGGAR_ADMIN_BIND=0.0.0.0`，并建议配合安全组白名单或 HTTPS 反向代理。

后台前端源码在 `web/admin`。Docker 构建会先执行 Vue build，再把产物嵌入 Go 二进制，运行时仍然只有一个容器：

```bash
cd web/admin
npm install
npm run build
```

## 新区扩容

管理后台的“区服配置”页面可以新增新区。填写服种、区服号，账号密码留空时会复用该服种默认账号；如果新区需要独立账号，也可以单独填写。WxPusher topicId 或 QQ 群号填入后会作为该新区的推送目标。保存后程序会写入动态扩容文件，并自动触发一次刷新登录，不需要重启容器。

动态区服默认保存到 `state/oldbeggar-expansion.json`，可用环境变量覆盖：

```bash
export OLDBEGGAR_EXPANSION_FILE=/app/state/oldbeggar-expansion.json
```

这个文件只保存后台新增的区服，不改写 `configs/config.local.yaml`。

## MySQL 存储

默认仍使用 `state/oldbeggar-state.json`。如果要把推送状态和历史事件写入 MySQL，设置：

```bash
export OLDBEGGAR_STORAGE_TYPE=mysql
export OLDBEGGAR_MYSQL_DSN='oldbeggar:强密码@tcp(mysql:3306)/oldbeggar?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
export OLDBEGGAR_STORAGE_AUTO_MIGRATE=1
```

如果 MySQL 也交给本项目的 Docker Compose 管理，可额外使用 `docker-compose.mysql.yml`：

```bash
export OLDBEGGAR_MYSQL_PASSWORD='换成强密码'
export OLDBEGGAR_MYSQL_DSN='oldbeggar:换成强密码@tcp(mysql:3306)/oldbeggar?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
docker compose -f docker-compose.server.yml -f docker-compose.mysql.yml up -d mysql app
```

程序会维护两张表：

- `oldbeggar_push_state`: 当前状态，用于防止当天重复推送，也用于管理后台当前列表。
- `oldbeggar_push_events`: 追加历史，每次 `Mark` 都插入一条；管理后台会读取最近 1000 条历史，后续可继续做统计。

首次切换到 MySQL 且 `oldbeggar_push_state` 为空时，程序会自动把现有 `state/oldbeggar-state.json` 的当前状态导入 MySQL，避免切库当天重复推送。JSON 里只有当前状态，没有完整历史，所以历史表会从导入时和后续运行开始累积。

初始化数据库示例：

```sql
CREATE DATABASE oldbeggar CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'oldbeggar'@'%' IDENTIFIED BY '换成强密码';
GRANT SELECT, INSERT, UPDATE, CREATE, ALTER, INDEX ON oldbeggar.* TO 'oldbeggar'@'%';
FLUSH PRIVILEGES;
```

## 架构

- `cmd/oldbeggar`: 程序入口，只处理配置、运行模式和退出信号。
- `internal/admin`: 管理后台静态资源、登录鉴权、状态接口和手动任务接口。
- `web/admin`: Vue 3 + Vite 管理后台源码。
- `internal/auth`: 登录流程，分别对应 `official`/`apple`、`h5`、`baozou`。
- `internal/game`: 区服列表解析、快速登录、事件状态查询、商店物品查询。
- `internal/protocol`: 游戏接口需要的 AES、DES、lz-string 压缩和签名。
- `internal/qq`: NapCat/OneBot HTTP 推送。
- `internal/wxpusher`: WxPusher topic 推送。
- `internal/state`: 每日推送状态持久化，避免重启后重复推送。
- `internal/runner`: 定时任务编排、并发查询和错误收敛。

## 本地运行

复制示例配置后填入环境变量或直接改成本地私有配置：

```powershell
Copy-Item configs/config.example.yaml configs/config.local.yaml
$env:OFFICIAL_USERNAME="你的账号"
$env:OFFICIAL_PASSWORD="你的密码或旧配置里的 md5:..."
$env:QQ_BOT_API_URL="http://127.0.0.1:3000"
$env:QQ_GROUP_ID="QQ群号"
$env:WXPUSHER_ENABLED="1"
$env:WXPUSHER_APP_TOKEN="你的 WxPusher appToken"
$env:WXPUSHER_TOPIC_ID_G1="g1 对应的 topicId"
go run ./cmd/oldbeggar -config configs/config.local.yaml -mode once
```

运行模式：

- `-mode run`: 常驻运行，按 cron 调度。
- `-mode once`: 启动、预检查、查询一次后退出。
- `-mode refresh`: 只刷新登录会话。
- `-mode check`: 使用现有会话查询商店；没有会话时会先刷新。

## 配置要点

`variants` 用 `kind` 区分登录策略，用 `server_rules` 把游戏区服名映射为配置里的区服编号。例如官方服规则把 `官方1` 映射为 `g1`，混服映射为 `h1`，苹果服映射为 `a1`，暴走服映射为 `b1`。

每个 `kind` 内置了对应的区服索引地址和 GetServerList payload；只有服务端协议变化时才需要显式配置 `server_index_url` 或 `server_list_payload`。

`notifications.watch_items` 是重点商品列表。命中时推送标题会改成重点商品提醒，内容会同时显示重点商品和全部商品。当前本地配置已加入 `铁剑令`。

`wxpusher` 用每个区一个 topic 的方式推送。先在 WxPusher 后台创建应用和 topic，把 `app_token` 填成应用 token，再把 `g1`、`h1`、`a1` 这类区服编号映射到对应的 `topicId`。映射可以直接写在 `wxpusher.topic_map`，也可以放到 JSON 文件后配置 `wxpusher.topic_map_file`，格式见 `configs/wxpusher-topics.example.json`。单个区也可以用环境变量覆盖，例如 `WXPUSHER_TOPIC_ID_G1=123456`。

如果只想用 WxPusher，不再发 QQ，把 `qq.api_url` 留空即可；如果保留 QQ 配置，程序会同时发 QQ 和 WxPusher，两个通道都成功后才记录当天已推送。

示例配置默认开启 `dry_run`，不会真正发 QQ 或 WxPusher。确认查询结果正常后再把 `app.dry_run` 改成 `false`，并把 `wxpusher.enabled` 改成 `true`；或设置环境变量 `DRY_RUN=1` 强制演练模式。
