# HerosTime 分支说明

这个仓库的 `main` 分支只作为项目导航页，不再存放具体程序代码。实际程序按分支维护。

## 推荐使用

优先使用 [`新程序`](https://github.com/ramoncjs3/HerosTime/tree/%E6%96%B0%E7%A8%8B%E5%BA%8F) 分支。

新程序是当前生产使用的 Go 重构版，把官方、苹果、H5、暴走几套老程序合并成一个配置驱动的长期运行服务，并带有管理后台。

## 分支区别

| 分支 | 类型 | 用途 | 特点 | 建议 |
| --- | --- | --- | --- | --- |
| `新程序` | 新版统一程序 | 当前生产主线 | 一个程序管理多服种；Vue 管理后台；支持账号密码登录；支持新区扩容；支持 WxPusher；可选 MySQL；默认兼容 JSON 状态文件；Docker 部署 | 后续维护和部署优先用这个 |
| `官方区服` | 老程序 | 官方服、混服推送 | 单独维护官方/混服逻辑；区服、账号、推送配置写在老配置里 | 只作历史参考，不建议继续扩展 |
| `苹果区服` | 老程序 | 苹果服推送 | 单独维护苹果服逻辑；和其他服种代码重复较多 | 只作历史参考 |
| `H5区服` | 老程序 | H5 服推送 | 单独维护 H5 登录和推送逻辑 | 只作历史参考 |
| `暴走区服` | 老程序 | 暴走服推送 | 单独维护暴走服登录和推送逻辑 | 只作历史参考 |

## 新程序怎么用

拉取新程序分支：

```bash
git clone -b 新程序 https://github.com/ramoncjs3/HerosTime.git
cd HerosTime
```

准备配置：

```bash
cp configs/config.example.yaml configs/config.local.yaml
```

然后编辑：

```text
configs/config.local.yaml
```

需要填入：

- 游戏账号密码
- WxPusher token 和 topic 映射
- 需要启用的服种和区服
- 管理后台账号密码

本地测试：

```bash
go test ./...
go run ./cmd/oldbeggar -config configs/config.local.yaml -mode run
```

Docker 部署：

```bash
docker compose -f docker-compose.server.yml up -d --build app
```

开启管理后台时设置环境变量：

```bash
export OLDBEGGAR_ADMIN_ENABLED=1
export OLDBEGGAR_ADMIN_USERNAME=admin
export OLDBEGGAR_ADMIN_PASSWORD='换成强密码'
docker compose -f docker-compose.server.yml up -d --build app
```

默认后台端口是 `8088`。

## 新区扩容

新程序不需要再改代码扩容。

进入管理后台：

```text
区服配置 -> 新区扩容
```

填写：

- 服种
- 新区区服号，例如 `h25`、`g4`、`b27`、`h5_16`
- 独立账号密码，可留空使用服种默认账号
- WxPusher topicId

保存后程序会写入：

```text
state/oldbeggar-expansion.json
```

并自动触发一次刷新登录，之后定时检查会带上新区。

## 老程序分支怎么用

如果必须查看或运行老程序，切到对应分支：

```bash
git clone -b 官方区服 https://github.com/ramoncjs3/HerosTime.git HerosTime-official
git clone -b 苹果区服 https://github.com/ramoncjs3/HerosTime.git HerosTime-apple
git clone -b H5区服 https://github.com/ramoncjs3/HerosTime.git HerosTime-h5
git clone -b 暴走区服 https://github.com/ramoncjs3/HerosTime.git HerosTime-baozou
```

老程序通常需要编辑：

```text
app/config/config.yaml
```

然后按老分支里的 Go 程序入口运行或编译。

老程序分支的问题：

- 每个服种一套代码，重复较多
- 新区扩容通常要改配置甚至改代码
- 没有统一管理后台
- 没有统一任务状态和推送记录页面
- 配置结构较旧，不适合继续长期扩展

## 后续维护原则

- 新功能统一加到 `新程序` 分支
- `main` 只维护说明文档
- 老程序分支只保留历史版本，不再主动扩展
- 私有配置、账号密码、token、运行状态文件不要提交到仓库
