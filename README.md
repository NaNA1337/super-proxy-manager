# Super-Proxy Manager 控制台

**Super-Proxy Manager** 是面向 [super-proxy](https://github.com/NaNA1337/super-proxy) 的现代化、独立多主机 Web 运维管理控制台（Control Panel / NOC Dashboard）。

项目采用前后端一体化单二进制架构，通过各 `super-proxy` 守护进程原生的 TLS Agent API 与集群通信。Manager 仅作为控制平面、多主机聚合器与客户端配置展示中心，绝不篡改底层守护进程的核心状态机（FSM）、节点评分与漏网保护规则。

```text
用户浏览器 (HTTPS)
      │
      ▼
┌──────────────────────────────────────────────────────────┐
│  Web Manager (super-proxy-web 单一二进制文件)              │
│  默认监听端口: :8443 (建议仅绑定 127.0.0.1 配合反代)        │
│  ├── 内嵌 React 18 + TypeScript + Tailwind 现代化 NOC 界面│
│  │   ├── 顶部多主机极速切换下拉框 (HOST: Tokyo-01 ▼)       │
│  │   ├── 主机集群管理与三步向导 (Hosts Fleet Wizard)       │
│  │   ├── 3秒 NOC 仪表盘 (实时健康度/出口节点/ASN/流量)     │
│  │   ├── 节点清册与打分透明度分析 (Nodes Inventory)        │
│  │   ├── 安全槽位控制器 (Slot Controller)                  │
│  │   ├── 策略路由监控与 Fail-Closed 漏网防护看板           │
│  │   ├── Prometheus 实时指标可视化图表                     │
│  │   ├── 实时系统事件流 (INFO / WARN / ERROR)              │
│  │   ├── 规范客户端配置中心 (VLESS / Clash / sing-box/Xray)│
│  │   ├── 多主机跨节点批量 ZIP 打包导出                     │
│  │   ├── 绑定主机的专属订阅管理器 (SHA-256 存储)           │
│  │   ├── 只读设置与系统架构探测器                          │
│  │   └── 安全变更审计日志流水 (Audit Trail)                │
│  └── 纯 Go 后端服务 (嵌入式前端资源，零外部依赖)            │
│      ├── 纯 Go 无 CGO SQLite 存储引擎 (WAL 模式)           │
│      ├── 首次启动高熵随机 Bootstrap 凭据初始化             │
│      ├── 强制首登修改管理员密码 (旧初始密码即刻作废)       │
│      ├── 主机凭据 AES-256-GCM 静态存储加密                 │
│      ├── TLS 证书指纹 (SHA-256) 绑定与自动探测校验        │
│      ├── SSRF 深度防御 (阻断私网/回环/云元数据/DNS重绑定)  │
│      ├── 订阅令牌脱敏存储与日志敏感字段屏蔽                │
│      └── 不可变多主机通信客户端池 (严格隔离)               │
└─────────────────────────────┬────────────────────────────┘
                              │
                    TLS 双向/指纹校验 + Token 认证
                    https://<daemon-host>:60000
                              ▼
┌──────────────────────────────────────────────────────────┐
│  super-proxy Daemon (核心网络引擎)                       │
│  ├── 状态机管理、节点健康打分、自动故障转移与信誉评估      │
│  ├── 策略路由分流 (fwmark 0x100-0x102, Table 100-102)     │
│  ├── Xray 核心 (SOCKS5 / VLESS Reality 入站)              │
│  └── OpenVPN Gate 多隧道并行出口出站                      │
└──────────────────────────────────────────────────────────┘
```

---

## 核心特性

- **多主机集中管控**：在一个控制台界面接入并切换管理全球各地域的 `super-proxy` 节点，顶部下拉框即选即切，前端状态与上下文绝对隔离。
- **零硬编码密码**：源码与配置文件中彻底杜绝任何硬编码管理员账号、默认密码或弱凭据。
- **首次启动安全引导 (Bootstrap)**：初次部署启动时，后端动态生成强随机初始密码并仅在终端控制台高亮输出一次。
- **强制修改初始密码**：管理员首次登录后必须立即设定强密码（不少于 12 位），旧临时密码立刻失效，杜绝遗忘改密风险。
- **纯 Go SQLite 本地持久化**：采用 `modernc.org/sqlite`（无 CGO 依赖，跨平台原生编译），启用 WAL 高并发模式，所有配置保存在 `data/super-proxy-manager.db`。
- **主机凭据静态加密**：各被管主机的 Agent Token 均在数据库中使用 AES-256-GCM 算法加密存储，日志与 API 响应完全脱敏。
- **严格 SSRF 深度防护**：内置网络层 IP 探测拦截器，从底层彻底阻断环回地址、RFC1918 私网网段、云厂商元数据接口（`169.254.169.254` 等）、CGNAT、IPv4-mapped IPv6 及 DNS 重绑定攻击。
- **TLS 证书指纹绑定 (Pinning)**：支持直连远程自签名 Agent，自动提取并绑定对端证书的 SHA-256 指纹。一旦证书被劫持或非预期变更，立即拒绝握手并报警。
- **规范客户端配置中心 (Source of Truth)**：配置数据直接读取自对应主机的 `/api/v1/client-config/all` 规范接口，前端仅做呈现与打包，不自行拼接或伪造 VLESS/Reality 参数。
  - **VLESS Reality / Vision**：标准分享链接与纯前端 Canvas 二维码生成。
  - **Clash Meta / Mihomo**：标准 YAML 节点配置块与一键下载。
  - **sing-box**：规范 Outbound JSON 对象。
  - **Xray-core**：完整出站配置 JSON。
  - **一键复制全部** / **跨主机批量 ZIP 打包导出**。
- **失效闭环防护 (Fail-Closed)**：若底层 Xray 停止或节点异常，Manager 立即清空缓存并呈现明确的不可用告警，严禁回退到 `127.0.0.1:1080` 或输出假配置。
- **单二进制极简部署**：编译输出仅 12MB 的独立二进制文件，前端页面全部由 Go `embed` 打包，部署无需预装 Node.js、Nginx 或外部数据库。

---

## 运行环境要求

- **操作系统**：Linux (Ubuntu 20.04+, Debian 11+, CentOS 7+, RHEL/Rocky/AlmaLinux 8+ 等), macOS, Windows
- **硬件配置**：
  - CPU: 1 核以上
  - 内存: 128 MB 以上（资源消耗极低）
  - 磁盘: 50 MB 以上可用空间
- **依赖软件**：
  - 运行预编译二进制：**零依赖**（无需安装 Node.js、Python 或外部数据库）
  - 从源码编译：Go 1.22+，Node.js 20+（仅重新构建前端时需要）

---

## 详细部署步骤

### 方式一：独立二进制直接运行（快速体验）

#### 1. 获取程序二进制
您可以直接使用编译好的 `bin/super-proxy-web`：
```bash
# 进入项目目录
cd /root/super-proxy-manager

# 如果需要手动编译最新版本：
go build -ldflags="-s -w" -o bin/super-proxy-web ./backend/cmd/server
```

#### 2. 创建运行目录并启动
```bash
# 启动程序（默认监听 0.0.0.0:8443，数据保存在当前目录下的 data/）
./bin/super-proxy-web -host 0.0.0.0 -port 8443 -data-dir ./data
```

#### 3. 记录终端生成的初始凭据
初次启动时，控制台将输出如下提示框：
```text
========================================================
SUPER-PROXY MANAGER FIRST-TIME SETUP
========================================================
Admin username: admin
Temporary password: <动态生成的32位随机临时密码>
IMPORTANT: This password will be invalidated after first password change.
Open: http://127.0.0.1:8443
========================================================
```
请复制该临时密码。

#### 4. 首次访问与密码初始化
1. 使用浏览器打开 `http://<服务器IP>:8443`。
2. 输入用户名 `admin` 与刚刚复制的临时密码。
3. 系统将弹出**强制修改密码**对话框，输入至少 12 位的强管理员密码。
4. 修改成功后，旧临时密码作废，使用新密码重新登录即可进入控制台。

---

### 方式二：生产环境 Systemd 服务化部署（强烈推荐）

在生产环境中，推荐将 Manager 部署为系统后台守护进程，限制运行权限，并在系统重启时自启动。

#### 1. 安装二进制文件
```bash
# 编译精简发布版二进制
go build -ldflags="-s -w" -o /usr/local/bin/super-proxy-web ./backend/cmd/server

# 赋予执行权限
chmod +x /usr/local/bin/super-proxy-web
```

#### 2. 创建数据目录与权限
```bash
# 创建专用数据目录
mkdir -p /var/lib/super-proxy-manager/data

# 设置安全的目录访问权限（仅允许 root 或运行用户读写）
chmod 700 /var/lib/super-proxy-manager/data
```

#### 3. 配置 Systemd 服务单元
创建服务文件 `/etc/systemd/system/super-proxy-web.service`：
```ini
[Unit]
Description=Super-Proxy Web Manager & Control Panel
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=/var/lib/super-proxy-manager
# 生产环境推荐仅监听本地回环 127.0.0.1，外部通过 Caddy/Nginx 代理加解密 HTTPS
ExecStart=/usr/local/bin/super-proxy-web -host 127.0.0.1 -port 8443 -data-dir /var/lib/super-proxy-manager/data
Restart=always
RestartSec=5s

# 安全沙箱加固
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=read-only
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

#### 4. 启动服务与开机自启
```bash
# 重载 systemd 配置
systemctl daemon-reload

# 设置开机自启并立即启动
systemctl enable --now super-proxy-web

# 查看服务运行状态
systemctl status super-proxy-web
```

#### 5. 获取首次启动密码
服务启动后，通过 `journalctl` 查看生成的临时 Bootstrap 密码：
```bash
journalctl -u super-proxy-web -n 30 --no-pager
```
在日志中找到 `Temporary password:` 行，复制密码并在浏览器完成首次修改。

---

### 方式三：配置 HTTPS 反向代理（Caddy / Nginx）

为了生产环境的数据传输安全，强烈建议通过反向代理配置域名并开启 HTTPS（TLS）加密访问。

#### 方案 A：使用 Caddy（极简自动化，推荐）
Caddy 会自动申请并续签 Let's Encrypt 证书。

编辑 `/etc/caddy/Caddyfile`：
```caddy
manager.yourdomain.com {
    # 自动申请证书并配置反向代理
    reverse_proxy 127.0.0.1:8443 {
        header_up Host {host}
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto https
    }

    # 日志输出配置
    log {
        output file /var/log/caddy/manager.access.log {
            roll_size 10mb
            roll_keep 5
        }
    }
}
```
重载 Caddy 即可：
```bash
systemctl reload caddy
```

#### 方案 B：使用 Nginx
如果已有 Nginx 环境，可配置如下虚拟主机：
```nginx
server {
    listen 80;
    server_name manager.yourdomain.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name manager.yourdomain.com;

    ssl_certificate     /etc/letsencrypt/live/manager.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/manager.yourdomain.com/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    location / {
        proxy_pass http://127.0.0.1:8443;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket 支持（系统事件推流）
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

---

## 运维参数与环境变量说明

`super-proxy-web` 支持通过命令行参数或环境变量进行配置：

| 命令行参数 | 对应环境变量 | 默认值 | 说明 |
| :--- | :--- | :--- | :--- |
| `-host <ip>` | `HOST` | `0.0.0.0` | 服务监听的 IP 地址（生产反代建议设为 `127.0.0.1`） |
| `-port <port>` | `PORT` | `8443` | 服务监听的 HTTP 端口号 |
| `-data-dir <path>` | `DATA_DIR` | `data` | SQLite 数据库文件与主密钥存放目录 |
| 无 | `MANAGER_MASTER_KEY` | *(自动生成)* | 用于 AES-256-GCM 加密存储的 32 字节十六进制主密钥。未设置时自动在 `data/.master.key` 生成并安全保存 |

---

## 业务使用实战指南

### 1. 添加被管主机 (Host Wizard)
1. 登录控制台后，进入左侧导航栏的 **Hosts Fleet**（主机集群）。
2. 点击右上角 **+ Add Host**，唤起三步添加向导：
   - **步骤 1：主机基础信息**
     - 主机标识 (Host ID)：如 `hk-edge-01`
     - 主机名称 (Host Name)：如 `Hong Kong Edge Alpha`
     - 所属区域 (Region/Country)：如 `Hong Kong (HK)`
   - **步骤 2：Agent 连接信息**
     - Agent 端点地址：填写远程 `super-proxy` 守护进程的 Agent API 地址，例如 `https://198.51.100.10:60000`
     - Agent 认证 Token：输入远程守护进程配置的 API 密钥
   - **步骤 3：安全指纹与连通性测试**
     - 点击 **Probe Certificate**，Manager 将自动探测远程 TLS 证书并提取 SHA-256 证书指纹进行 Pinning 绑定。
     - 点击 **Test Connection**，验证端到端安全连接、状态机状态及节点上报情况。
3. 点击 **Save Host** 保存，该主机凭据随即在 SQLite 中被 AES-256-GCM 密文持久化。

### 2. 多主机实时切换
- 在页面顶部导航栏正中，始终常驻 **`HOST: <当前主机名称> ▼`** 下拉框。
- 点击即可一键切换到任意集群主机，仪表盘、节点打分、槽位控制、策略路由和客户端配置均立刻切换至该主机的独立视图，不同主机数据严格隔离，杜绝串扰。

### 3. 获取客户端配置 (Client Config)
进入左侧 **Share Links**（客户端配置中心）：
- 系统自动调用当前选中主机的 `/api/v1/client-config/all` 规范接口。
- 支持单节点与全节点模式：
  - **VLESS Reality**：点击 **[Copy]** 直接复制链接；点击 **[QR]** 弹出离线高清二维码供手机客户端扫码导入。
  - **Clash Meta**：点击 **[Download]** 获得预设策略组与完整 outbound 的 `clash-meta.yaml`。
  - **sing-box**：点击 **[Download]** 获得规范 JSON 出站配置 `sing-box.json`。
  - **Xray-core**：点击 **[Download]** 获得 `xray.json`。
  - **Copy All**：一键以格式化多协议文本导出当前节点所有格式配置。
- **批量跨主机导出 (Batch ZIP)**：点击右上角 **Batch Export**，可跨主机勾选多个出站节点，一键生成结构清晰、防路径穿越的安全 ZIP 压缩包下载。

---

## 源码构建与自动化测试

### 1. 完整重新构建流程
```bash
# 1. 编译前端生产静态资源
cd frontend
npm ci
npm run build
cd ..

# 2. 将前端构建产物嵌入并编译后端
go build -ldflags="-s -w" -o bin/super-proxy-web ./backend/cmd/server
```

### 2. 运行自动化测试矩阵

#### 后端测试与 Race 检测
```bash
# 执行全部后端单元测试与集成测试
go test -count=1 ./...

# 启用 Go Race 检测器验证并发线程安全（必须 PASS）
go test -count=1 -race ./...
```

#### 前端类型与静态检查
```bash
cd frontend
# 静态代码与类型检查
npm run typecheck
npm run lint
```

#### 真实浏览器 Web UI E2E 自动化测试 (Playwright)
项目内置了一套针对真实 Chromium 浏览器的端到端自动化测试套件：
```bash
cd frontend
# 安装 Playwright 所需浏览器内核及系统依赖（首次执行）
npx playwright install chromium --with-deps

# 执行端到端浏览器自动化测试
npm run test:e2e
```
测试矩阵全自动覆盖：首次引导、密码重置、会话生命周期、多主机隔离、规范配置生成、运行时不可用 Fail-Closed、跨主机批量导出、SSRF 全网段阻断防御与 TLS 指纹劫持拦截。

---

## 数据备份与故障恢复 (FAQ)

### Q1: 数据库与密钥如何备份？
数据目录（默认为 `data/` 或 `/var/lib/super-proxy-manager/data/`）下仅包含两个核心文件：
1. `super-proxy-manager.db`：SQLite 主数据库（包含用户表、主机表、审计日志）。
2. `.master.key`：AES-256-GCM 主密钥（若未通过环境变量 `MANAGER_MASTER_KEY` 指定）。

**备份命令**：
```bash
# 备份整个数据目录即可完成完整迁移
tar -czvf super-proxy-manager-backup-$(date +%F).tar.gz /var/lib/super-proxy-manager/data/
```

### Q2: 忘记管理员密码如何重置？
停止服务后，可以在数据目录下删除管理员密码记录，或清空并重新初始化：
```bash
systemctl stop super-proxy-web
# 若需重新进行首次引导初始化：
# 启动时指定空数据目录或备份后重建数据库，系统将自动重新输出一次性临时密码
```

### Q3: 添加主机时测试连接失败？
1. **网络与安全组**：确认被管主机的 `60000` 端口（或对应 Agent 端口）已在云防火墙和安全组对 Manager IP 放行。
2. **SSRF 拦截规则**：出于安全防护，Manager 严禁添加 `127.0.0.1`、`localhost`、`10.0.0.0/8`、`192.168.0.0/16` 等内网及回环地址（除非在专门的测试开发环境下）。请确保 Agent 填写公网有效 IP 或外部可解析域名。
3. **证书指纹匹配**：如果目标 Agent 的证书曾重新生成，需重新点击 **Probe Certificate** 更新指纹，否则 TLS Pinning 机制会主动断开连接以防中间人攻击。

---

## 开源协议

本项目采用 [Apache-2.0](LICENSE) 开源许可证。
