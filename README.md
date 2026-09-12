# Super-Proxy Manager

Super-Proxy Manager 是 [Super-Proxy Core](https://github.com/NaNA1337/super-proxy) 的多主机 Web 控制台。

```text
浏览器 → Manager :8443 → 多台 Core Agent :60000
                              ↓
                       Xray / OpenVPN
```

Manager 负责主机管理、出口状态、手动切换、指标查看和客户端配置导出。它不运行 Xray、OpenVPN，也不需要应用配置文件。

当前版本：[v1.0.5](https://github.com/NaNA1337/super-proxy-manager/releases/tag/v1.0.5)

Dashboard 的 `Measured Multi-Connection Capacity` 是所有活动槽位独立测速值之和。Core 会把独立的新连接按轮询顺序分到三个出口；单个 TCP 连接保持一个出口，因此单任务测速仍受单条 TUN 限制。

## 快速安装

支持使用 systemd 的 Ubuntu/Debian `amd64` 和 `arm64`。发布版是静态单文件程序，运行时不需要 Go、Node.js、npm 或 SQLite。

安装下载工具：

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates wget
```

下载、校验并安装到 `/usr/local/bin`：

```bash
VERSION=1.0.5
ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64|arm64) ;;
  *) echo "不支持的架构: $ARCH"; exit 1 ;;
esac

INSTALL_DIR=/tmp/super-proxy-manager-install
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

wget -O "super-proxy-web-linux-${ARCH}" \
  "https://github.com/NaNA1337/super-proxy-manager/releases/download/v${VERSION}/super-proxy-web-linux-${ARCH}"
wget -O checksums.txt \
  "https://github.com/NaNA1337/super-proxy-manager/releases/download/v${VERSION}/checksums.txt"

grep "  super-proxy-web-linux-${ARCH}$" checksums.txt | sha256sum -c -
sudo install -m 755 "super-proxy-web-linux-${ARCH}" /usr/local/bin/super-proxy-web
super-proxy-web -version
```

正确输出应包含：

```text
super-proxy-manager 1.0.5
```

## 直接启动

Manager 不读取 YAML 或 JSON 配置。裸启动即可使用：

```bash
sudo install -d -m 700 /var/lib/super-proxy-manager/data
sudo super-proxy-web
```

默认监听 `0.0.0.0:8443`，数据库和主密钥固定保存在 `/var/lib/super-proxy-manager/data`。从任何目录执行裸启动都会使用同一个数据库。

浏览器访问：

```text
http://Manager服务器IP:8443
```

如果 Core Agent 使用 `127.0.0.1`、`10.x`、`172.16-31.x` 或 `192.168.x` 地址，启动时增加 `-allow-private-hosts`：

```bash
sudo super-proxy-web -allow-private-hosts
```

公网 Core Agent 不需要该参数。即使启用该参数，云厂商元数据和 link-local 地址仍会被禁止。

8443 默认是 HTTP。只建议在受信任网络中直接开放；公网部署应使用 Caddy 或 Nginx 提供 HTTPS。

## 首次登录

首次启动会创建管理员，并在终端输出一次临时密码：

```text
Admin username: admin
Temporary password: 随机生成的密码
```

使用 `admin` 登录后必须立即修改密码，新密码至少 12 位。临时密码不会再次显示。

忘记密码时保留数据库并生成新的临时密码：

```bash
sudo systemctl stop super-proxy-web
sudo super-proxy-web -reset-admin-password
sudo systemctl start super-proxy-web
```

命令会输出新的单次临时密码，旧密码和旧会话立即失效；已添加的主机、Token 和其他数据不会被删除。

## 添加 Core 主机

进入 **Hosts Fleet → Add Host**：

| 字段 | 示例 |
| --- | --- |
| Name | `Tokyo-01` |
| Address | `proxy.example.com` |
| Agent URL | `https://core.example.com:60000` |
| Token | Core `/etc/super-proxy/config.yaml` 中的 `api.key` |
| TLS fingerprint | Core Agent 证书的 SHA-256 指纹 |

同机连接时，Agent URL 使用：

```text
https://127.0.0.1:60000
```

异机连接时，Core 的 `api.listen` 必须监听 Manager 可以访问的地址。Agent URL 必须使用 `https://`，TCP/60000 应只允许 Manager 来源访问。

在 Core 服务器查看证书指纹：

```bash
sudo openssl x509 \
  -in /etc/super-proxy/cert.pem \
  -noout -fingerprint -sha256
```

点击 **Test Connection**，核对指纹后保存主机。Manager 会把 Agent Token 加密保存在自己的数据目录中，不会返回给浏览器。

## 使用 systemd

下载并安装仓库自带的服务文件：

```bash
VERSION=1.0.5
wget -O /tmp/super-proxy-web.service \
  "https://raw.githubusercontent.com/NaNA1337/super-proxy-manager/v${VERSION}/deploy/super-proxy-web.service"

sudo install -d -m 750 /var/lib/super-proxy-manager
sudo install -d -m 700 /var/lib/super-proxy-manager/data
sudo install -m 644 \
  /tmp/super-proxy-web.service \
  /etc/systemd/system/super-proxy-web.service

sudo systemctl daemon-reload
sudo systemctl enable --now super-proxy-web
sudo systemctl status super-proxy-web --no-pager
```

服务默认监听 `127.0.0.1:8443`。从自己的电脑建立 SSH 隧道：

```bash
ssh -N -L 8443:127.0.0.1:8443 用户名@Manager服务器IP
```

然后打开 `http://127.0.0.1:8443`。

查看首次登录密码或运行日志：

```bash
sudo journalctl -u super-proxy-web -n 100 --no-pager
```

如果 systemd 服务需要连接内网 Core，编辑服务：

```bash
sudo systemctl edit --full super-proxy-web
```

在 `ExecStart` 命令末尾增加 `-allow-private-hosts`，然后重启：

```bash
sudo systemctl daemon-reload
sudo systemctl restart super-proxy-web
```

## 启动参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-host` | `0.0.0.0` | Manager Web 监听地址 |
| `-port` | `8443` | Manager Web HTTP 端口 |
| `-data-dir` | `/var/lib/super-proxy-manager/data` | 数据库和主密钥目录 |
| `-allow-private-hosts` | 关闭 | 允许连接回环和私网 Core Agent |
| `-reset-admin-password` | 关闭 | 生成新的管理员临时密码并退出 |
| `-version` | — | 显示版本 |

查看帮助：

```bash
super-proxy-web -h
```

`HOST`、`PORT`、`DATA_DIR`、`ALLOW_PRIVATE_HOSTS` 环境变量仍然兼容，但常规部署直接使用上面的命令参数即可。

## 数据和备份

默认服务数据目录：

```text
/var/lib/super-proxy-manager/data
├── super-proxy-manager.db
└── .master.key
```

如果曾使用 v1.0.2 或更早版本裸启动，旧数据可能位于启动目录下的 `data`。停止 Manager 后，把整个旧目录迁移到默认位置，数据库和 `.master.key` 必须一起移动：

```bash
sudo systemctl stop super-proxy-web
sudo install -d -m 700 /var/lib/super-proxy-manager/data
sudo cp -a /原启动目录/data/. /var/lib/super-proxy-manager/data/
sudo systemctl start super-proxy-web
```

`.master.key` 用于加密保存的 Agent Token，必须与数据库一起备份：

```bash
sudo systemctl stop super-proxy-web
sudo cp -a \
  /var/lib/super-proxy-manager/data \
  /你的备份目录/manager-data
sudo systemctl start super-proxy-web
```

当前订阅记录保存在内存中，Manager 重启后需要重新创建订阅链接。

## 升级

先备份数据，然后重新执行“快速安装”中的下载、校验和安装命令，最后重启：

```bash
sudo systemctl restart super-proxy-web
super-proxy-web -version
```

## 卸载

下载发布版自带的卸载脚本：

```bash
VERSION=1.0.5
wget -O /tmp/super-proxy-manager-uninstall.sh \
  "https://github.com/NaNA1337/super-proxy-manager/releases/download/v${VERSION}/uninstall.sh"
sudo bash /tmp/super-proxy-manager-uninstall.sh
```

默认只删除 systemd 服务和 `/usr/local/bin/super-proxy-web`，数据仍保存在 `/var/lib/super-proxy-manager`，以后重装可以继续使用。

确认不再需要数据库、主机 Token 和主密钥时，执行完全清理并按提示输入 `DELETE`：

```bash
sudo bash /tmp/super-proxy-manager-uninstall.sh --purge-data
```

无人值守完全清理可增加 `--yes`。v1.0.2 或更早版本可能在手动启动目录留下相对 `data`，卸载脚本不会自动删除无法确认归属的目录。

## 从源码构建

源码构建需要：

- Go 1.26 或更高版本
- Node.js 22 或更高版本
- npm
- Git
- Bash
- CA certificates

构建：

```bash
git clone https://github.com/NaNA1337/super-proxy-manager.git
cd super-proxy-manager
./scripts/build.sh
```

输出文件：

```text
bin/super-proxy-web
```

安装并启动：

```bash
sudo install -m 755 bin/super-proxy-web /usr/local/bin/super-proxy-web
sudo super-proxy-web
```

运行测试：

```bash
go test ./...
go test -race ./...
go vet ./...
npm --prefix frontend run typecheck
```

浏览器端到端测试还需要 Playwright Chromium：

```bash
cd frontend
npx playwright install --with-deps chromium
npm run test:e2e
```

## 常见问题

| 问题 | 检查 |
| --- | --- |
| 页面打不开 | 检查 `-host`、`-port` 和防火墙 |
| Manager 拒绝内网 Agent | 增加 `-allow-private-hosts` |
| Agent `Connection refused` | 检查 Core 的 `api.listen` 和 TCP/60000 |
| Agent 返回 401/403 | 检查 Core 当前 `api.key` |
| TLS 指纹不匹配 | 确认证书是否被重新生成 |
| 重启后主机 Token 无法解密 | 恢复与数据库配套的 `.master.key` |
| 忘记管理员密码 | 停止服务后运行 `sudo super-proxy-web -reset-admin-password` |

- [详细操作教程](docs/tutorial.md)
- [Manager 与 Agent API 审计](docs/web-manager-api-audit.md)
- [Super-Proxy Core](https://github.com/NaNA1337/super-proxy)
