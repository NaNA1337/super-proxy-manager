# 从零接入核心与日常操作

## 1. 准备并启动两个服务

核心默认路径 `/root/super-proxy`，Manager 默认路径 `/root/super-proxy-manager`。路径不同可以替换。先按核心 README 创建固定密钥配置、启动核心，再用本仓库 `scripts/build.sh` 构建和启动 Manager。

同机最小端口分配：Xray `443`，Agent `60000`，Manager HTTP `8443`。浏览器通过 SSH 转发或另一个 HTTPS 端口进入 Manager。将网页的 HTTPS 反向代理也放在同机 443 会与 Xray 冲突。

## 2. 初始化管理员

查看 Manager 本次启动日志，使用 `admin` 和临时密码登录。页面要求修改密码时完成修改；不要寻找固定默认密码。刷新后应保持会话。退出后使用新密码登录验证。

如果初始化过却忘记密码，不要直接删除数据目录；该目录还保存主机信息和解密密钥，应先备份并按你的恢复流程处理。

## 3. 建立主机信任

在 Hosts Fleet 点击 Add Host，输入显示名、公网地址、Agent HTTPS URL 和 Token。使用 Test Connection 查看检测结果，在核心本机读取证书 SHA-256 指纹并核对。保存后设为默认。

- `Connection refused`：先检查核心日志、60000 监听和地址是否正确。
- `401`：未提供 Token；`403`：Token 错误。用核心当前配置或环境变量覆盖值。
- 私网被拒绝：同机/内网场景在 Manager 进程设置 `ALLOW_PRIVATE_HOSTS=true` 后重启。
- 指纹不匹配：核实核心是否更换证书，确认后更新。不要直接把校验关闭。

## 4. 检查出口状态

在导航栏选主机后进入 Dashboard、Nodes、Slots Manager。默认 3 个活动槽位，其 Linux 表号与 mark 为 100、101、102。空出口时仍可正常浏览页面。

候选 IP 是 VPN 出口地址；公网分享入口地址是核心服务器地址，两者不是一回事。发现节点不等于节点可用；只有通过隧道、出口 IP 和健康检测后才应作为活动出口。

手动切换步骤：选择槽位，选择真实候选节点，提交后等待操作进度。核心返回 202 只代表请求已接受。成功终态 `ACTIVE`，失败终态 `FAILED`，并显示原因。409 通常表示槽位繁忙、节点已使用或资格不符。任务记录存放在核心内存，重启后无法继续轮询旧任务。

## 5. 导出配置并验证客户端

Share Links 选择主机后读取核心配置。没有活动 VPN 节点但 VLESS 入站已准备好时，也能显示网关级配置；这有助于先配置客户端，但仍需等实际出口就绪后才能通信。

- VLESS：复制 URI 或扫描 QR。
- Clash Meta：下载 YAML，导入支持 Reality 的客户端。
- sing-box / Xray：下载 JSON，先用客户端自己的配置检查命令验证。
- ZIP：选择主机/节点后下载；上游不可用项会带状态说明。

要验证完整链路，请从另一台客户端导入配置，请求一个你控制的 HTTPS 测试地址，确认出口 IP 与 VPN 出口相符，再测试切换和失效后的阻断。本轮自动联调只覆盖本地管理与配置，不替代这一步。

## 6. 使用订阅

创建订阅时选择目标主机并立即复制返回的完整 URL；不要把 Agent 管理 Token 分发给普通客户端。订阅 URL 自身是访问凭据。撤销后用旧 URL 请求应返回 403。

当前订阅在进程内存中，重启 Manager 后需要重新创建。区域/协议筛选是配置筛选，不会建立新的地区专用入站；核心 schema v1 返回的国家为网关元数据，不能视为逐出口国家保证。

## 7. 更新和回滚

先备份配置及数据目录，再构建并安装新 Manager 二进制。`scripts/build.sh` 同步前端嵌入资源。重启后检查登录、主机列表、测试连接、Share Links；订阅重建后通知实际使用者。保留旧二进制和对应数据库备份以便回滚。

## 8. 可重复联调

```bash
cd /root/super-proxy-manager/frontend
npx playwright install chromium
cd /root/super-proxy
sudo bash scripts/test-manager.sh /root/super-proxy-manager
```

所有测试状态保存在临时目录，进程退出后清理。网络名称空间中仅使用回环网络，因此不接触开发服务器正在使用的路由或公网 443。
