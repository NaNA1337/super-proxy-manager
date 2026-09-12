# Super-Proxy Manager

管理多个 [Super-Proxy](https://github.com/NaNA1337/super-proxy) 核心实例的 Web 控制台。Go 后端提供登录、主机注册、Agent API 转发、客户端配置下载和订阅；React 前端编译后嵌入同一个二进制。

核心负责 OpenVPN、Xray 和 Linux 路由。Manager 本身不建立 VPN。浏览器只访问 Manager，由 Manager 携带加密存储的 Agent Token 请求核心。

当前版本已与核心 v1.1.2 及其后续修复联调。真实联调入口见 [联调教程](docs/tutorial.md)，检查记录见核心仓库 [可用性报告](../super-proxy/docs/usability-report.md)。

## 源码构建和首次启动

依赖：Go 1.26+（见 go.mod）、Node.js 22、npm。运行 Manager 无需 root。

```bash
# 在本仓库根目录执行；会安装锁文件依赖、编译前端、同步嵌入资源并编译 Go。
./scripts/build.sh

# 同机 Agent 地址为 https://127.0.0.1:60000 时需允许私网地址。
ALLOW_PRIVATE_HOSTS=true ./bin/super-proxy-web \
  -host 127.0.0.1 -port 8443 -data-dir ./data
```

默认后端是 **HTTP**，端口名 `8443` 不代表已开启 TLS。开发时打开 `http://127.0.0.1:8443`；远程服务器可从个人电脑建立 SSH 转发：

```bash
ssh -L 8443:127.0.0.1:8443 用户名@服务器IPv4
```

首次运行在日志中输出账号 `admin` 和随机临时密码。登录后强制修改密码，之后使用新密码。不存在通用默认密码。请保留数据目录；删除数据会触发重新初始化。

## 添加核心主机

先按核心仓库 README 部署核心，确认 Agent 为 HTTPS/60000，VLESS 为 443。进入 **Hosts Fleet → Add Host**：

| 字段 | 含义 / 示例 |
| --- | --- |
| Name | 显示名，如 `东京出口` |
| Address | 核心的公网 IP 或域名，如 `proxy.example.com` |
| Agent URL | 同机 `https://127.0.0.1:60000`；分机填管理地址 |
| Token | 核心 `config.yaml` 中 `api.key`，或其 `XRAY_MANAGER_API_KEY` 覆盖值 |
| TLS fingerprint | Agent 自签名证书的 SHA-256 指纹 |

在核心服务器核对指纹：

```bash
openssl x509 -in /etc/super-proxy/cert.pem -noout -fingerprint -sha256
```

测试连接，核对证书后保存并设置默认主机。`ALLOW_PRIVATE_HOSTS=true` 仅在你确实需要接入内网或回环地址时启用。Token 在服务器端存储，浏览器主机列表不会返回原始 Token。

## 日常使用

1. **Dashboard / Hosts Fleet**：选择目标主机、检查在线状态。在线只说明管理 API 可响应。
2. **Nodes / Slots Manager**：检查候选池和活动隧道。没有可用节点时，空列表是有效状态。手动切换需选择实际合格节点；任务终态为 `ACTIVE` / `FAILED`。
3. **Policy Routing / Prom Metrics**：查看核心路由信息与指标。实际内核状态还需核心的 `super-proxy diagnose routing` 和系统命令验证。
4. **Share Links**：查看、复制、下载 VLESS、Clash Meta、sing-box、Xray 和订阅配置；支持 QR 与批量 ZIP。所有配置内容来自核心 `/api/v1/client-config/all`，Manager 只适配响应结构。
5. **Subscriptions**：创建链接后立即保存；原始订阅 Token 只显示一次。撤销后返回 403。当前订阅记录保存在内存，Manager 重启后需要重建。
6. **Settings / Audit Log / Events & Logs**：密码修改、配置查看和当前进程审计信息。当前日志不是持久化的集中日志平台。

一个核心对外提供一个 VLESS 入口。不同 VPN 出口显示的分享配置可能相同，不表示客户端被固定路由到所选国家或 IP。分享配置可生成也不代表已有活动出口；真实代理请求是最终验收标准。

## 部署为服务

```bash
sudo install -m 755 bin/super-proxy-web /usr/local/bin/super-proxy-web
sudo install -d -m 750 /var/lib/super-proxy-manager
sudo install -m 644 deploy/super-proxy-web.service /etc/systemd/system/super-proxy-web.service
sudo systemctl daemon-reload
# 同机/内网 Agent 场景：在 override 中写入以下两行：
# [Service]
# Environment=ALLOW_PRIVATE_HOSTS=true
sudo systemctl edit super-proxy-web
sudo systemctl enable --now super-proxy-web
sudo journalctl -u super-proxy-web -n 60 --no-pager
```

前置 HTTPS 反向代理参考 [Caddy 配置](deploy/Caddyfile.example)。**同一公网 IP 上核心 Xray 已占用 443 时，Manager 的反向代理不能再绑定该 IP 的 443**；使用另一台服务器、另一个 IP，或为 Manager 选择独立 HTTPS 端口。不要为了 Manager 停止核心的客户端入口。

浏览器需要可访问 Manager 的 HTTPS 地址；Agent 的 60000 只允许管理来源。反向代理需转发 `X-Forwarded-Proto: https`，使订阅链接生成正确的协议。

## 配置和备份

| 配置 | 默认 / 说明 |
| --- | --- |
| `-host` / `HOST` | `0.0.0.0`；部署建议显式绑定回环 |
| `-port` / `PORT` | `8443`，HTTP |
| `-data-dir` / `DATA_DIR` | `data` |
| `ALLOW_PRIVATE_HOSTS` | 默认拒绝私网 Agent；`true` 明确放行 |
| `MANAGER_MASTER_KEY` | 可选，建议 64 位十六进制编码的 32 字节密钥 |

环境变量覆盖相应命令行值。未设置主密钥时，数据目录自动生成 `.master.key`。备份必须同时包含 SQLite 数据库和该密钥，否则无法解密 Agent Token。最简单的可靠备份流程为停止 Manager、复制整个数据目录、重新启动。订阅和内存审计不包含在数据库备份中。

不要只构建前端后就重启旧 Go 二进制：前端资源已嵌入，必须重新运行 `scripts/build.sh` 并安装新二进制。

## 开发、测试与联调

```bash
go test ./...
go test -race ./...
go vet ./...
npm --prefix frontend run typecheck
npm --prefix frontend run lint
# lint 当前是 TypeScript 类型检查，不是独立 ESLint 规则集。
cd frontend
npx playwright install chromium
npm run test:e2e
```

上面的浏览器套件默认使用模拟 Agent。真正的双项目检查在核心目录执行：

```bash
cd /root/super-proxy
sudo bash scripts/test-manager.sh /root/super-proxy-manager
```

它重新构建两个项目，在隔离网络及挂载名称空间中运行真实核心、Xray、Manager 和 Chromium，覆盖登录改密、证书固定、接口转发、五种配置、ZIP、订阅撤销、页面空状态和核心停止后的失效处理。它不依赖公网 VPN Gate，也不宣称公网隧道或 Reality 外网握手已通过。

本地前端开发可运行 `npm --prefix frontend run dev`；Vite 将 `/api` 和 `/sub` 转发至本机 HTTP/8443。生产服务使用构建后的 Go 二进制。
