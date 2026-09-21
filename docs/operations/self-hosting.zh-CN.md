# 部署 Piik Server

[English](./self-hosting.md) · 简体中文 · [文档导航](../README.md)

Piik Server 将网页、房间管理和可选的媒体转发打包在**一个服务端程序**中。
解压后即可运行，房间数据保存在 SQLite 中。

## 先在本机试用

以下服务端操作在 Linux x64 环境中执行。

此仓库是基于 [TNTcraftHIM/Piik](https://github.com/TNTcraftHIM/Piik) 的自用适配版本，保留 MIT 许可证。
上游下载包和镜像不包含这里的修复。请检出本仓库的修复分支，在干净工作区中使用
Node（见 `.node-version`）和 Go 1.26.8 构建，输出目录必须位于仓库外且尚不存在：

```sh
npm ci
node scripts/package-server-release.mjs ../piik-runtime
```

将输出的 `piik-<revision>-runtime.tar.gz` 解压到服务目录，在该目录运行：

```sh
./piik-server
```

打开 `http://localhost:8787` 即可试用。需要让朋友通过互联网访问时，
继续完成下方配置。如果只想在自己的电脑上临时开房间，可直接使用 [Piik App](../guide/getting-started.zh-CN.md#用-piik-app-分享)。

## 使用 Docker Compose

在已安装 Docker Compose v2 的 Linux x64 服务器上，从本仓库构建包含修复的镜像，
再复制部署文件到服务目录。以下命令从干净的仓库根目录执行，输出目录须尚不存在：

```sh
npm ci
node scripts/package-server-release.mjs ../piik-container --container-image piik:self-hosted
mkdir -p ../piik-deploy
cp deploy/container/compose.yaml ../piik-deploy/
cp deploy/container/.env.example ../piik-deploy/.env
cd ../piik-deploy
```

将 `.env` 中的 `share.example.com` 换成自己的域名，并设置非空的私有 `SITE_ACCESS_PASSWORD`，然后启动：

```sh
docker compose run --rm piik --check-config
docker compose up -d
```

本地 `piik:self-hosted` 镜像包含网页、信令、STUN 和可选 SFU。
接着完成下方的 [HTTPS 配置](#2-配置-https)和[端口放行](#3-放行端口并检查)。
默认使用 P2P；如需 SFU 兜底，按 `.env` 中的说明启用即可。
请保留 `piik-data` 数据卷，房间数据和可选诊断文件都保存在其中。
更新与备份见[容器维护说明](./service-management.zh-CN.md#容器)。

## 对外提供服务

准备一台 Linux x64 服务器，以及一个指向服务器公网 IP 的域名。
下文以 `share.example.com` 为例，请替换成自己的域名。

### 1. 配置并启动 Piik

在程序旁新建 `.env` 文件：

```dotenv
PIIK_ENV=production
LISTEN_HOST=127.0.0.1
PUBLIC_BASE_URL=https://share.example.com
STUN_URLS=stun:share.example.com:3478
MAX_VIEWERS_PER_ROOM=20
SITE_ACCESS_PASSWORD=
TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128
```

先填写 `SITE_ACCESS_PASSWORD`（不要提交真实口令），再使用普通用户在该目录执行：

```sh
./piik-server --check-config
./piik-server
```

程序会自动读取 `.env`，并将房间数据保存在工作目录的 `rooms.sqlite` 中。
生产模式会拒绝空白站点口令。新浏览器默认创建仅邀请可加入的私密房间；
已有房间与明确保存的访问设置保持原样。持有邀请链接的观众仍可直接加入该房间。
登录和建房在应用内按客户端 IP 限流，超限返回 `429`；具体额度见[配置边界](../standards/configuration.md#bounds)。
上面的代理信任配置仅适用于同机代理；容器中应填写应用实际看到的代理来源 IP/CIDR。
代理应覆盖客户端传入的 `X-Forwarded-For`，不要信任任意来源或填写整个公网。

`MAX_VIEWERS_PER_ROOM` 可设置每房观众上限，不含房主，支持 `1..20`，修改后重启生效。
人数越多，对网络和转发资源的需求也可能增加。默认值及 App 房间的区别见
[人数限制](../standards/configuration.md#room-capacity)。

### 2. 配置 HTTPS

如果已有 HTTPS 反向代理，将请求转发到 `127.0.0.1:8787`，并启用 WebSocket 支持。
如果还没有，可以[安装 Caddy](https://caddyserver.com/docs/install)，在 Caddyfile 中加入：

```text
share.example.com {
    reverse_proxy 127.0.0.1:8787
}
```

重新加载 Caddy。域名解析正确且 TCP 80/443 可访问时，它会自动申请和续期证书。
具体操作见 [Caddy 的 HTTPS 代理说明](https://caddyserver.com/docs/quick-starts/reverse-proxy#https)。

### 3. 放行端口并检查

在服务器防火墙和云平台安全组中放行 **TCP 80/443**（HTTPS）和 **UDP 3478**（STUN）。
TCP 8787 仅供本机反向代理访问。STUN 域名需要直接解析到服务器，不能只经过 CDN 的 HTTP 代理。

打开 `https://share.example.com/healthz`，应返回 `{"status":"ok"}`。
随后打开站点，分享一个画面，并用另一台设备加入验证。
上述基础配置使用 P2P 传输，参与者之间需要可用的 UDP 通路。

## 可选：启用媒体兜底

在 `.env` 中添加 `SFU_UDP_PORT=7882`，放行 UDP 7882，再重启 Piik，即可启用自动 SFU 兜底。
如果服务器处于 NAT 后方，还需将 `SFU_PUBLIC_IP` 设置为外部可达的公网 IPv4 地址。
这项功能由同一个服务端程序提供。
房主需要在开始分享前关闭 **隐私模式**，才会允许使用这条线路。
直连和 SFU 媒体都需要可用的 UDP 通路。
当前未实现 TURN/TCP/TLS 中继；屏蔽 UDP 的网络不能保证连通。

房间 HTTP API、`/signal` WebSocket、自建 STUN 和可选 SFU 接口均保留。
正常 P2P 媒体由参与者传输，服务器承担页面、鉴权和信令；启用 SFU 后才可能承担媒体带宽。
此部署方式无需依赖上游演示站或 cloudflared，后者只用于 App 的公网邀请模式。

## 长期运行与更新

需要开机启动时，参阅 [systemd 与容器配置](./service-management.zh-CN.md)。
更新时保留 `.env` 和 `rooms.sqlite`：停止服务、备份房间数据、替换新版程序，再启动并检查健康状态和房间访问。
涉及数据格式变化的版本，请先阅读发布说明。

此适配仓库默认不自动发布版本或网站。仅在明确需要时设置 GitHub Actions 仓库变量
`PIIK_RELEASES_ENABLED=true` 或 `PIIK_WEBSITE_ENABLED=true`，并配置相应发布权限。

[全部配置与端口](../standards/configuration.md) · [维护者发版工具](../deployment.md)
