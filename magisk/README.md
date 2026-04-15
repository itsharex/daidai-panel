# 呆呆面板 Magisk 模块

通过 Magisk / KernelSU / APatch 在已 Root 的 Android 设备上运行呆呆面板，面板在开机后自动启动，浏览器访问 `http://127.0.0.1:5700` 即可使用。

> 本模块无需 Termux、无需 Docker，面板后端为单一静态 Go 二进制，直接以 root 身份托管，数据持久化在 `/data/adb/daidai-panel/`。

---

## 系统要求

- 已 Root 的 Android 设备，至少满足以下任意 root 方案之一：
  - Magisk **v24.0+**
  - KernelSU
  - APatch
- Android 8.0 (API 26) 及以上
- CPU 架构：`arm64-v8a` 或 `x86_64`（默认发布 arm64 版本）

## 安装

1. 下载 `daidai-panel-magisk-vX.Y.Z.zip`（或按下文自行构建）
2. 打开 Magisk 管理器 →「模块」→「从本地安装」，选择该 ZIP
3. 安装完成后重启手机
4. 手机上打开任意浏览器，访问 `http://127.0.0.1:5700`
5. 按提示初始化管理员账号

## 目录结构

```
/data/adb/modules/daidai-panel/        ← 模块本体（跟随模块卸载）
  ├── module.prop
  ├── daidai-server                    ← 后端二进制
  ├── ddp                              ← 内置命令
  ├── service.sh / customize.sh / uninstall.sh
  └── web/                             ← 打包后的前端静态文件

/data/adb/daidai-panel/                ← 持久化数据目录（不随卸载清除）
  ├── config.yaml
  ├── daidai.db                        ← SQLite 数据库
  ├── service.log / server.log         ← 启动与运行日志
  └── data/
      ├── scripts/  logs/  backups/  deps/
```

## 在模块卡片内一键更新

模块 `module.prop` 里已经填好了 `updateJson`：

```
updateJson=https://github.com/linzixuanzz/daidai-panel/releases/latest/download/update.json
```

这是 **GitHub Release 的稳定跳转地址**，会自动指向"当前最新一次 Release"里随附的 `update.json`。因此：

1. 每次仓库推送新的 `vX.Y.Z` tag，工作流会自动:
   - 编译 arm64 + amd64 后端
   - 打包 `daidai-panel-magisk-vX.Y.Z.zip`
   - 生成指向本次 Release 的 `update.json`（含版本号 / versionCode / zipUrl / changelog）
   - 将这两个文件一起上传到 Release
2. 用户手机上已安装的老版本，打开 Magisk / KernelSU / APatch 管理器时，会自动拉取 `update.json`，对比 `versionCode` 发现比本地新 → 模块卡片上出现「**更新**」按钮
3. 用户点击按钮 → 管理器自动下载 `zipUrl`、执行安装流程（等同于手动「从本地安装 ZIP」）
4. 重启手机完成升级；数据目录 `/data/adb/daidai-panel/` 不变，数据库、脚本、日志都保留

> 说明：需要管理器版本支持 `updateJson`（Magisk v24.0+、KernelSU、APatch 新版均支持）。如果你自己 fork 了本项目发版，请把 `module.prop` 里的 `linzixuanzz/daidai-panel` 替换成自己的仓库路径即可。

### 手动触发更新检查

部分管理器默认只在打开模块列表时刷新一次。想立即触发，可以：

- **Magisk**：在「模块」页面下拉刷新
- **KernelSU / APatch**：在「模块」页面点右上角的刷新图标

如果希望强制下载最新 ZIP（比如想跳过 versionCode 比较），也可以直接从 Release 页下载 ZIP 手动安装，数据目录同样不会被清。

---

## 脚本运行时说明（重要）

呆呆面板本身是一个调度器和 Web 面板，**真正执行脚本**（Python、Node.js、Shell、TypeScript、Go…）依赖系统里装了对应的解释器 / 运行时。在 Docker Alpine / Debian 镜像里这些是内置的，但**Android 系统本身不带 Python / Node / bash / git**，所以需要额外提供运行时才能跑脚本。

模块对此做了三件事：

1. **只打包面板本体**：模块 ZIP 只包含 `daidai-server` 后端 + `ddp` 命令 + 前端静态文件，不把 Python / Node 一起塞进去（否则体积暴涨到 100MB+ 还不一定匹配你的系统）。
2. **启动时自动合并常见运行时路径**：`service.sh` 会把下列路径加进面板进程的 `PATH`，只要其中任一路径里有 `python3` / `node`，面板就能直接调用：
   - `/data/adb/daidai-panel/bin/`（你自己放的静态二进制）
   - `/data/data/com.termux/files/usr/bin/`（Termux 的标准安装位置）
   - `/data/data/com.termux/files/usr/local/bin/`
   - 系统自带的 `/system/bin`、`/system/xbin`、`/vendor/bin`
   Termux 的 `lib` 目录也会追加到 `LD_LIBRARY_PATH`，动态链接的二进制不会报 `library not found`。
3. **提供自检脚本 + action 按钮**：点模块卡片的「运行」按钮，或手动执行
   ```bash
   su -c "sh /data/adb/modules/daidai-panel/scripts/check-runtimes.sh"
   ```
   会逐项打印 `python3 / node / npm / git / curl ...` 是否可用，缺哪个一目了然。

### 推荐搭配方式（任选一种）

| 方案 | 做法 | 适合 |
|------|------|------|
| **A. Termux（推荐）** | 从 F-Droid 装 Termux，执行 `pkg install python nodejs git curl`，然后重启手机/面板 | 想完整支持 Python / Node / Shell 脚本 |
| **B. 自带静态二进制** | 把自己编译或下载的静态版 `python3` / `node` 放到 `/data/adb/daidai-panel/bin/`，授予 `chmod 0755` | 不想装 Termux、想完全独立 |
| **C. 只跑 Shell 脚本** | 什么都不装，默认就能跑 `sh` 脚本（Android 自带）| 只用来定时跑命令、调 API、做备份 |

> 提示：面板里「依赖管理」页会通过 `pip` / `npm` 工作，前提是系统里存在对应的 `pip` / `npm`。Termux 方案下 `pkg install python nodejs` 就都带上了。

---

## 在管理器内查看日志（推荐）

模块内置 `action.sh`，在 **Magisk v26+ / KernelSU / APatch** 的模块列表里，呆呆面板条目右侧会出现「运行 / Action」按钮。点击后会直接在管理器弹窗里输出：

- 当前面板进程状态、PID
- `5700 / 5701` 端口监听情况
- `ddp status` 结果（版本、资源占用、任务数等）
- `service.log` 最近 60 行（启动过程日志）
- `server.log` 最近 60 行（后端运行日志）

忘记密码 / 启动异常时都可以先点这里看输出，不用 adb 连线。

## 常用操作

在 `adb shell` 或手机上的 Shell App（如 Termux、MT 管理器的 Shell）中：

```bash
# 查看服务日志
su -c "tail -f /data/adb/daidai-panel/service.log"

# 查看后端日志
su -c "tail -f /data/adb/daidai-panel/server.log"

# 使用内置 ddp 命令（已加入模块目录，可通过绝对路径调用）
su -c "/data/adb/modules/daidai-panel/ddp status"
su -c "/data/adb/modules/daidai-panel/ddp list-users"
su -c "/data/adb/modules/daidai-panel/ddp reset-password admin NewPass123"

# 手动重启面板
su -c "pkill -f daidai-server; sh /data/adb/modules/daidai-panel/service.sh &"
```

## 忘记密码怎么办？

直接用内置命令即可，无需卸载模块：

```bash
su
/data/adb/modules/daidai-panel/ddp list-users
/data/adb/modules/daidai-panel/ddp reset-password <用户名> <新密码>
```

## 修改端口

- 前端默认监听端口与 Docker 版不同。为避免跟系统服务抢端口，面板在手机上直接以后端形式监听 `5700`；`/data/adb/daidai-panel/config.yaml` 里的 `server.port` 即是浏览器访问端口。
- 修改后重启手机或执行 `su -c "sh /data/adb/modules/daidai-panel/service.sh"` 生效。

## 对系统的影响（安装不会动系统分区）

本模块是**纯用户态 / 非侵入式**的：

| 类别 | 是否触碰 | 说明 |
|------|----------|------|
| `/system` 分区 | ❌ | 不修改、不替换任何系统文件 |
| `system.prop` / `sepolicy.rule` | ❌ | 不写系统属性、不加 SELinux 规则 |
| 应用安装 / 广告 / 服务伪装 | ❌ | 不安装 APK、不注册账户、不做任何后台伪装 |
| 网络监听 | ⚠️ | 仅监听 `127.0.0.1:5700`，不对外网暴露 |
| 写入位置 | ✅ | 只写 `/data/adb/modules/daidai-panel/`（模块本体）和 `/data/adb/daidai-panel/`（数据） |

也就是说，面板运行的全部「痕迹」都集中在 `/data/adb/daidai-panel/` 和 Magisk 自己的模块目录里；系统分区、其他 APP、网络策略都不会被改动。禁用模块后，进程也会停止监听。

## 卸载（默认彻底清理，不留痕迹）

1. 在 Magisk / KernelSU / APatch 管理器中移除本模块
2. 重启手机

重启完成后会自动做以下事情：

- 停止仍在运行的 `daidai-server` 进程
- 删除模块本体 `/data/adb/modules/daidai-panel/`（由 Magisk 框架负责）
- 删除持久化数据目录 `/data/adb/daidai-panel/`（数据库、脚本、日志、备份，**默认全部清除**）

卸载后设备上不会残留任何呆呆面板相关文件。

### 想保留数据以便日后重装？

在卸载前创建一个保留标记即可，`uninstall.sh` 看到标记就会跳过数据目录清理：

```bash
su -c "touch /data/adb/daidai-panel/.keep_on_uninstall"
```

后续如果又想彻底删掉保留的数据：

```bash
su -c "rm -rf /data/adb/daidai-panel"
```

### 想卸载前先导出一份备份？

```bash
su -c "/data/adb/modules/daidai-panel/ddp backup create --name before-uninstall"
# 备份生成在 /data/adb/daidai-panel/data/backups/
# 先拷出来再卸载即可
```

## 本地构建

在项目根目录执行：

```bash
# 仅打包 arm64（默认）
bash Magisk/build.sh 2.0.6

# 同时打包 arm64 + x86_64
bash Magisk/build.sh 2.0.6 all
```

构建产物：`dist/daidai-panel-magisk-v<版本>.zip`。

> 首次构建会自动执行 `npm ci && npm run build` 生成前端，需要 Node.js 20+。

## FAQ

**Q: 浏览器打不开 `http://127.0.0.1:5700`？**
- 在 `adb shell` 里 `su -c "netstat -ltnp | grep 5700"` 确认进程监听情况；
- 查看 `/data/adb/daidai-panel/server.log`；
- 部分 MIUI / 一加 OS 默认阻止后台网络访问，需要把你使用的浏览器加入后台白名单，或使用 Chrome / Via 等不被冻结的浏览器。

**Q: 重启后模块没启动？**
- 打开 Magisk → 模块列表，确认「呆呆面板」处于已启用状态；
- 查看 `/data/adb/daidai-panel/service.log`；
- 如手机开启了严苛的省电/冻结策略，请把 Magisk Daemon / `magiskd` 加入白名单。

**Q: 可以用 Docker 一键更新吗？**
- 面具版直接在宿主机（Android）上跑，不依赖 Docker。升级请下载最新版 ZIP 并重新安装，数据目录不会被清除。
