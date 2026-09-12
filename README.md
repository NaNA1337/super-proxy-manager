# Super-Proxy Manager

Super-Proxy Manager 是 [Super-Proxy](https://github.com/NaNA1337/super-proxy) 的 Web 管理控制台。一个 Manager 可以管理多个核心服务器，查看 VPN Gate 节点、活动出口和策略路由，并导出 VLESS Reality、Clash Meta、sing-box、Xray 和订阅配置。

```text
浏览器 → Manager HTTP/8443 → Agent HTTPS/60000 → Super-Proxy 核心
                                      ↓
                          Xray + OpenVPN + 策略路由
```

Manager 只提供管理界面，不运行 Xray 或 OpenVPN。当前稳定版为 [v1.0.1](https://github.com/NaNA1337/super-proxy-manager/releases/tag/v1.0.1)，与 Super-Proxy Core v1.1.3 联调通过。

## 最快安装

适用于使用 systemd 的 Ubuntu/Debian `amd64` 或 `arm64` 服务器。发布版是包含前端资源的静态二进制，不需要安装 Go、Node.js、npm 或 SQLite。

### 1. 安装运行依赖

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates wget coreutils grep sed
```

| 依赖 | 用途 |
| --- | --- |
| `ca-certificates` | 验证 GitHub 的 HTTPS 证书 |
| `wget` | 下载发布文件 |
| `coreutils`、`grep`、`sed` | 校验文件、安装二进制和读取首次登录日志；标准系统通常已自带 |
| `systemd` | 将 Manager 作为后台服务运行，Ubuntu/Debian Server 默认已安装 |

按需使用的工具：管理员电脑通过 SSH 隧道访问时需要 OpenSSH Client；在核心服务器手动查看证书指纹时需要 OpenSSL；公网 HTTPS 访问才需要 Caddy 或 Nginx。这些都不是 Manager 二进制的运行依赖。

### 2. 用 wget 下载并安装到 PATH

下面的命令会自动识别 `amd64` 或 `arm64`，校验 SHA-256，然后把程序安装为 `/usr/local/bin/super-proxy-web`：

```bash
VERSION=1.0.1
ARCH="$(dpkg --print-architecture)"
case "$ARCH" in amd64|arm64) ;; *) echo "不支持的架构: $ARCH"; exit 1 ;; esac

sudo install -d -m 755 /usr/local/bin
mkdir -p /tmp/super-proxy-manager-install
cd /tmp/super-proxy-manager-install

wget -O "super-proxy-web-linux-${ARCH}" \
  "https://github.com/NaNA1337/super-proxy-manager/releases/download/v${VERSION}/super-proxy-web-linux-${ARCH}"
wget -O checksums.txt \
  "https://github.com/NaNA1337/super-proxy-manager/releases/download/v${VERSION}/checksums.txt"

grep "  super-proxy-web-linux-${ARCH}$" checksums.txt | sha256sum -c -
sudo install -m 755 "super-proxy-web-linux-${ARCH}" /usr/local/bin/super-proxy-web
super-proxy-web -version
```

预期版本输出包含 `super-proxy-manager 1.0.1`。

### 3. 安装 systemd 服务

```bash
VERSION=1.0.1
wget -O /tmp/super-proxy-web.service \
  "https://raw.githubusercontent.com/NaNA1337/super-proxy-manager/v${VERSION}/deploy/super-proxy-web.service"

sudo install -d -m 750 /var/lib/super-proxy-manager
sudo install -d -m 700 /var/lib/super-proxy-manager/data
sudo install -m 644 /tmp/super-proxy-web.service /etc/systemd/system/super-proxy-web.service
sudo systemctl daemon-reload
```

Manager 默认监听 `127.0.0.1:8443`。如果 Manager 需要连接同机的 `127.0.0.1:60000`，或者核心服务器的内网地址，执行：

```bash
sudo systemctl edit super-proxy-web
```

在编辑器中写入并保存：

```ini
[Service]
Environment=ALLOW_PRIVATE_HOSTS=true
```

核心 Agent 使用公网地址时不需要设置这个选项。

启动 Manager：

```bash
sudo systemctl enable --now super-proxy-web
sudo systemctl status super-proxy-web --no-pager
sudo journalctl -u super-proxy-web -n 80 --no-pager
```

### 4. 首次登录

首次启动会在日志中输出唯一的临时密码：

```bash
sudo journalctl -u super-proxy-web -b --no-pager | \
  sed -n '/SUPER-PROXY MANAGER FIRST-TIME SETUP/,+7p'
```

默认账号是 `admin`。第一次登录后必须设置至少 12 位的新密码；系统没有通用默认密码。

最快且安全的访问方式是从自己的电脑建立 SSH 隧道：

```bash
ssh -N -L 8443:127.0.0.1:8443 用户名@Manager服务器IP
```

然后在本机浏览器打开：

```text
http://127.0.0.1:8443
```

8443 默认提供 HTTP。不要把它直接暴露到公网；公网访问请使用 HTTPS 反向代理。

## 连接 Super-Proxy 核心

先确认核心服务器已经启动：

- 客户端入口：TCP/443。
- Agent API：HTTPS/60000。
- 核心 `config.yaml` 已设置 `xray.vless.public_address`。
- 异机连接时，核心 `api.listen` 已绑定内网地址或 `0.0.0.0`。
- 核心防火墙只允许 Manager 来源访问 TCP/60000。

核心和 Manager 不在同一台服务器时，核心配置示例：

```yaml
api:
  listen: 0.0.0.0
  port: 60000
  key: 保持你的随机API密钥
```

核心服务器只允许 Manager 的固定来源 IP：

```bash
sudo ufw allow from Manager服务器IP to any port 60000 proto tcp
sudo systemctl restart super-proxy
```

在 Manager 中进入 **Hosts Fleet → Add Host**：

| 字段 | 填写内容 |
| --- | --- |
| Name | 自定义名称，例如 `东京出口` |
| Address | 核心的公网 IP 或域名，用于客户端连接 |
| Agent URL | 同机 `https://127.0.0.1:60000`；异机 `https://核心管理IP或域名:60000` |
| Token | 核心 `/etc/super-proxy/config.yaml` 中的 `api.key` |
| TLS fingerprint | 核心 Agent 证书的 SHA-256 指纹 |

在核心服务器查看证书指纹：

```bash
sudo openssl x509 -in /etc/super-proxy/cert.pem -noout -fingerprint -sha256
```

点击测试连接，核对 Manager 检测到的指纹，保存并设为默认主机。Agent URL 必须使用 `https://`。

## 日常使用

1. **Dashboard / Hosts Fleet**：选择核心服务器并检查在线状态。
2. **Nodes / Slots Manager**：查看候选节点、3 个活动出口和备用隧道，或发起手动切换。
3. **Policy Routing / Prom Metrics**：查看路由标记、隧道接口和核心指标。
4. **Share Links**：复制或下载 VLESS、Clash Meta、sing-box、Xray 和 Base64 订阅。
5. **Subscriptions**：创建和撤销订阅链接。原始订阅 Token 只显示一次。
6. **Settings / Audit Log**：修改密码并查看当前 Manager 进程的审计事件。

一个核心只有一个公网 VLESS Reality 入口。页面中的逻辑槽位对应不同 VPN 出口，但分享链接本身不会固定到某个 VPN Gate 节点。请用真实 HTTPS 代理请求验证最终出口。

当前订阅记录保存在内存中，Manager 重启后需要重新创建订阅链接。

## HTTPS 公网访问

Manager 自身提供 HTTP，生产环境应在前面使用 Caddy、Nginx 或其他 HTTPS 反向代理，并转发：

```text
X-Forwarded-Proto: https
```

仓库提供 [Caddy 示例](deploy/Caddyfile.example)。如果 Manager 和核心共用同一个公网 IP，核心 Xray 已占用 TCP/443，反向代理不能再绑定同一 IP 的 443。可选择：

- 继续使用 SSH 隧道访问 Manager。
- 给 Manager 使用另一台服务器或另一个公网 IP。
- 使用独立的 HTTPS 端口，并相应配置证书和防火墙。

## 配置项

| 命令行参数 | 环境变量 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-host` | `HOST` | `0.0.0.0` | Web 监听地址；systemd 服务显式使用 `127.0.0.1` |
| `-port` | `PORT` | `8443` | Web HTTP 端口 |
| `-data-dir` | `DATA_DIR` | `data` | SQLite 数据库和主密钥目录 |
| — | `ALLOW_PRIVATE_HOSTS` | `false` | 是否允许 Manager 请求回环或私网 Agent |
| — | `MANAGER_MASTER_KEY` | 自动生成 | 32 字节主密钥，推荐使用 64 位十六进制字符串 |

查看全部启动参数：

```bash
super-proxy-web -h
```

未设置 `MANAGER_MASTER_KEY` 时，Manager 会在数据目录生成权限为 0600 的 `.master.key`。Agent Token 使用该密钥加密后写入 SQLite。

## 升级

升级二进制不会覆盖数据目录：

```bash
sudo cp -a /var/lib/super-proxy-manager \
  "/var/lib/super-proxy-manager.backup.$(date +%Y%m%d-%H%M%S)"

VERSION=1.0.1
ARCH="$(dpkg --print-architecture)"
mkdir -p /tmp/super-proxy-manager-install
cd /tmp/super-proxy-manager-install
wget -O "super-proxy-web-linux-${ARCH}" \
  "https://github.com/NaNA1337/super-proxy-manager/releases/download/v${VERSION}/super-proxy-web-linux-${ARCH}"
wget -O checksums.txt \
  "https://github.com/NaNA1337/super-proxy-manager/releases/download/v${VERSION}/checksums.txt"
grep "  super-proxy-web-linux-${ARCH}$" checksums.txt | sha256sum -c -

sudo install -m 755 "super-proxy-web-linux-${ARCH}" /usr/local/bin/super-proxy-web
sudo systemctl restart super-proxy-web
super-proxy-web -version
```

## 备份与恢复

数据库、自动生成的 `.master.key` 和加密后的 Agent Token 都在 `/var/lib/super-proxy-manager/data`。备份必须包含整个数据目录：

```bash
sudo systemctl stop super-proxy-web
sudo cp -a /var/lib/super-proxy-manager/data /安全备份路径/manager-data
sudo systemctl start super-proxy-web
```

恢复时停止服务，用备份目录替换数据目录后再启动。缺少 `.master.key` 会导致已保存的 Agent Token 无法解密。

## 从源码构建

源码构建需要：

| 依赖 | 版本 / 用途 |
| --- | --- |
| Git | 获取源码 |
| Bash | 执行 `scripts/build.sh` |
| Go | 1.26 或更高版本，编译后端 |
| Node.js | 22 或更高版本，编译前端 |
| npm | 随 Node.js 安装，按 lockfile 安装前端依赖 |
| CA certificates | 下载 Go module 和 npm 包 |

确认工具版本：

```bash
sudo apt-get update
sudo apt-get install -y bash git ca-certificates

git --version
go version
node --version
npm --version
```

构建并安装：

```bash
git clone https://github.com/NaNA1337/super-proxy-manager.git
cd super-proxy-manager
./scripts/build.sh

./bin/super-proxy-web -version
sudo install -m 755 ./bin/super-proxy-web /usr/local/bin/super-proxy-web
```

`scripts/build.sh` 会执行 `npm ci`、TypeScript/Vite 前端构建、同步嵌入资源并用 `CGO_ENABLED=0` 编译 Go 二进制。前端已经嵌入程序，修改前端后必须重新构建整个二进制。

浏览器端到端测试还需要 Playwright Chromium 及其系统依赖：

```bash
cd frontend
npx playwright install --with-deps chromium
npm run test:e2e
```

运行 `go test -race ./...` 还需要 C 编译器；Ubuntu/Debian 可安装 `build-essential`。制作 Release 另外需要 `diffutils`、`tar`、`gzip` 和 `coreutils`。

## 开发与联调

```bash
go test ./...
go test -race ./...
go vet ./...
npm --prefix frontend run typecheck
npm --prefix frontend run lint
npm --prefix frontend run test:e2e
```

与真实核心进行完整联调时，在核心仓库执行：

```bash
cd /root/super-proxy
sudo bash scripts/test-manager.sh /root/super-proxy-manager
sudo python3 tests/live/check.py /root/super-proxy-manager
```

- [详细操作教程](docs/tutorial.md)
- [Manager 与 Agent API 审计](docs/web-manager-api-audit.md)
- [Super-Proxy Core 部署文档](https://github.com/NaNA1337/super-proxy)
