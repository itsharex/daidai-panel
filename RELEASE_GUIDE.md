# 呆呆面板 发布指南

每次发布新版本时，按以下步骤依次执行。

---

## 1. 更新版本号

修改 `server/handler/version.go` 中的版本号：

```go
var Version = "x.x.x"
```

## 2. 提交代码并推送

```bash
git add -A
git commit -m "release: vx.x.x"
git push origin main
```

## 3. 编译二进制文件

```bash
# Linux AMD64
SET GOOS=linux
SET GOARCH=amd64
SET CGO_ENABLED=0
go build -ldflags "-X daidai-panel/handler.Version=x.x.x" -o daidai-server-linux-amd64 ./server

# Linux ARM64
SET GOOS=linux
SET GOARCH=arm64
SET CGO_ENABLED=0
go build -ldflags "-X daidai-panel/handler.Version=x.x.x" -o daidai-server-linux-arm64 ./server

# Windows AMD64
SET GOOS=windows
SET GOARCH=amd64
SET CGO_ENABLED=0
go build -ldflags "-X daidai-panel/handler.Version=x.x.x" -o daidai-panel.exe ./server
```

> `-ldflags` 将版本号注入到编译产物中，运行时可正确显示版本。

## 4. 创建 GitHub Release

```bash
gh release create vx.x.x --title "vx.x.x" --latest --notes "更新日志内容" daidai-server-linux-amd64 daidai-server-linux-arm64 daidai-panel.exe
```

| 参数 | 说明 |
|------|------|
| `vx.x.x` | Git tag，自动创建 |
| `--title` | Release 标题 |
| `--latest` | 标记为最新版本 |
| `--notes` | 更新日志（支持 Markdown） |
| 末尾文件列表 | 作为附件上传的二进制文件 |

## 5. 构建并推送 Docker 镜像

```bash
# 构建镜像（同时打 latest 和版本号两个 tag）
docker build --no-cache --build-arg VERSION=x.x.x -t linzixuanzz/daidai-panel:latest -t linzixuanzz/daidai-panel:x.x.x .
docker build --no-cache --build-arg VERSION=1.1.0 -t linzixuanzz/daidai-panel:latest -t linzixuanzz/daidai-panel:1.1.0 .

# 推送到 Docker Hub
docker push linzixuanzz/daidai-panel:latest
docker push linzixuanzz/daidai-panel:x.x.x
```

| 参数 | 说明 |
|------|------|
| `--build-arg VERSION=x.x.x` | 构建时注入版本号到镜像 |
| `-t` | 给镜像打标签，打两个确保 latest 始终指向最新 |

## 6. 验证

```bash
# 检查 Release
gh release view vx.x.x

# 检查 Docker 镜像
docker pull linzixuanzz/daidai-panel:x.x.x
docker run --rm linzixuanzz/daidai-panel:x.x.x --version
```

---

## 快速参考（替换所有 x.x.x 为实际版本号）

```bash
# 一键流程（Windows CMD）
SET VER=x.x.x

SET GOOS=linux&& SET GOARCH=amd64&& SET CGO_ENABLED=0&& go build -ldflags "-X daidai-panel/handler.Version=%VER%" -o daidai-server-linux-amd64 ./server
SET GOOS=linux&& SET GOARCH=arm64&& SET CGO_ENABLED=0&& go build -ldflags "-X daidai-panel/handler.Version=%VER%" -o daidai-server-linux-arm64 ./server
SET GOOS=windows&& SET GOARCH=amd64&& SET CGO_ENABLED=0&& go build -ldflags "-X daidai-panel/handler.Version=%VER%" -o daidai-panel.exe ./server

gh release create v%VER% --title "v%VER%" --latest --notes "更新日志" daidai-server-linux-amd64 daidai-server-linux-arm64 daidai-panel.exe

docker build --build-arg VERSION=%VER% -t linzixuanzz/daidai-panel:latest -t linzixuanzz/daidai-panel:%VER% .
docker push linzixuanzz/daidai-panel:latest
docker push linzixuanzz/daidai-panel:%VER%
```
