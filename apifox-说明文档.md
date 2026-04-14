<p align="center">
  <img src="https://raw.githubusercontent.com/linzixuanzz/daidai-panel/main/images/图标.png" alt="呆呆面板" width="120">
</p>

<h1 align="center">呆呆面板</h1>

<p align="center">
  <em>轻量、现代的定时任务管理面板，Docker 一键部署，开箱即用</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3-4FC08D?logo=vue.js&logoColor=white" alt="Vue3">
  <img src="https://img.shields.io/badge/Element%20Plus-2.x-409EFF?logo=element&logoColor=white" alt="Element Plus">
  <img src="https://img.shields.io/badge/SQLite-3-003B57?logo=sqlite&logoColor=white" alt="SQLite">
  <img src="https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white" alt="Docker">
</p>

---

呆呆面板 (Daidai Panel) 是一款轻量级定时任务管理平台，采用 Go (Gin) + Vue3 (Element Plus) + SQLite 架构，专注于脚本托管与自动化任务调度。支持 Python、Node.js、Shell、TypeScript、Go 等多语言脚本的定时执行与可视化管理，内置 18 种消息推送渠道、订阅管理、环境变量、依赖管理、Open API 等功能。Docker 一键部署，开箱即用。

## 功能特性

- **定时任务** — Cron 表达式调度，支持重试、超时、任务依赖、前后置钩子
- **脚本管理** — 在线代码编辑器，支持 Python、Node.js、Shell、TypeScript、Go，拖拽移动文件
- **执行日志** — SSE 实时日志流，历史日志查看与自动清理
- **环境变量** — 分组管理、拖拽排序、批量导入导出（兼容青龙格式）
- **订阅管理** — 自动从 Git 仓库拉取脚本，支持定期同步
- **依赖管理** — 可视化安装/卸载 Python (pip) 和 Node.js (npm) 依赖
- **通知推送** — Bark、Telegram、Server酱、企业微信、钉钉、飞书等 18 种渠道
- **开放 API** — App Key / App Secret 认证，支持第三方系统对接
- **系统安全** — 双因素认证 (2FA)、IP 白名单、登录日志、会话管理
- **数据备份** — 一键备份与恢复，导出全部数据
- **系统监控** — 实时 CPU / 内存 / 磁盘监控，任务执行趋势统计

## 效果图

| 功能 | 截图 |
|------|------|
| 登录页面 | ![登录](https://raw.githubusercontent.com/linzixuanzz/daidai-panel/main/images/登录.png) |
| 仪表盘 | ![仪表盘](https://raw.githubusercontent.com/linzixuanzz/daidai-panel/main/images/仪表盘.png) |
| 定时任务 | ![定时任务](https://raw.githubusercontent.com/linzixuanzz/daidai-panel/main/images/定时任务.png) |
| 脚本管理 | ![脚本管理](https://raw.githubusercontent.com/linzixuanzz/daidai-panel/main/images/脚本管理.png) |
| 环境变量 | ![环境变量](https://raw.githubusercontent.com/linzixuanzz/daidai-panel/main/images/环境变量.png) |
| 订阅管理 | ![订阅管理](https://raw.githubusercontent.com/linzixuanzz/daidai-panel/main/images/订阅管理.png) |
| 消息通知 | ![消息通知](https://raw.githubusercontent.com/linzixuanzz/daidai-panel/main/images/消息通知.png) |
| 依赖管理 | ![依赖管理](https://raw.githubusercontent.com/linzixuanzz/daidai-panel/main/images/依赖管理.png) |

## 快速部署

### Docker Compose（推荐）

```yaml
services:
  daidai-panel:
    image: docker.1ms.run/linzixuanzz/daidai-panel:latest
    container_name: daidai-panel
    restart: unless-stopped
    ports:
      - "5700:5700"
    volumes:
      - ./Dumb-Panel:/app/Dumb-Panel
      - /var/run/docker.sock:/var/run/docker.sock  
    environment:
      - TZ=Asia/Shanghai
      - CONTAINER_NAME=daidai-panel
      - IMAGE_NAME=docker.1ms.run/linzixuanzz/daidai-panel:latest
```

```bash
docker compose up -d
```

启动后访问：`http://localhost:5700`

### Docker Run

```bash
docker run -d \
  --pull=always \
  --name daidai-panel \
  --restart unless-stopped \
  -p 5700:5700 \
  -v $(pwd)/Dumb-Panel:/app/Dumb-Panel \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e TZ=Asia/Shanghai \
  -e CONTAINER_NAME=daidai-panel \
  -e IMAGE_NAME=docker.1ms.run/linzixuanzz/daidai-panel:latest \
  docker.1ms.run/linzixuanzz/daidai-panel:latest
```

启动后访问：`http://localhost:5700`，首次使用需要初始化管理员账号。

## 内置命令

容器内内置了 `ddp` 命令，用于在终端里直接做常见运维操作：

```bash
docker exec -it daidai-panel ddp help
docker exec -it daidai-panel ddp status
docker exec -it daidai-panel ddp check
docker exec -it daidai-panel ddp logs --lines 200
docker exec -it daidai-panel ddp script list
docker exec -it daidai-panel ddp env list
docker exec -it daidai-panel ddp task list --status running
docker exec -it daidai-panel ddp backup create --name nightly
docker exec -it daidai-panel ddp restart
docker exec -it daidai-panel ddp update
```

## 数据目录

```
./Dumb-Panel/
├── daidai.db          # SQLite 数据库
├── .jwt_secret        # 自动生成的 JWT 密钥
├── panel.log          # 面板运行日志
├── deps/              # Python / Node.js 依赖目录
├── scripts/           # 脚本文件存储
├── logs/              # 执行日志
└── backups/           # 数据备份
```

## 技术栈

| 层 | 技术 |
|----|------|
| 前端 | Vue 3 + TypeScript + Element Plus + Pinia + Vite |
| 后端 | Go (Gin) + GORM + SQLite |
| 部署 | Nginx + Go Binary，Docker 单镜像（AMD64 / ARM64） |

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `TZ` | 时区 | `Asia/Shanghai` |
| `DATA_DIR` | 数据存储目录 | `/app/Dumb-Panel` |
| `DB_PATH` | 数据库路径 | `${DATA_DIR}/daidai.db` |
| `PANEL_PORT` | 容器内 Nginx 监听端口 | `5700` |
| `SERVER_PORT` | Go 服务端口 | `5701` |

## 多架构支持

镜像同时支持 `linux/amd64` 和 `linux/arm64`，可在 x86 服务器和 ARM 设备（如树莓派、Oracle ARM 云服务器）上直接运行。

## 致谢

本项目的开发离不开以下优秀的开源项目：

- **[白虎面板 (Baihu Panel)](https://github.com/engigu/baihu-panel)** — 后端框架架构参考
- **[青龙面板 (Qinglong)](https://github.com/whyour/qinglong)** — 功能设计参考

## 链接

- [GitHub 仓库](https://github.com/linzixuanzz/daidai-panel)
- [问题反馈](https://github.com/linzixuanzz/daidai-panel/issues)

## LICENSE

Copyright © 2026, [linzixuanzz](https://github.com/linzixuanzz). Released under the [MIT](https://github.com/linzixuanzz/daidai-panel/blob/main/LICENSE).
