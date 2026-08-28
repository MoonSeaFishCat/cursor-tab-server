# Cursor Tab Server

一个带调用 API Key、审计日志和 React 管理控制台的 Cursor 服务代理。它使用你自己的 Cursor 登录令牌调用固定白名单中的上游接口；它不会绕过 Cursor 的订阅、额度、计费或权限校验。

## 安全模型

- 代理请求默认必须带 `X-API-Key`；也可使用查询参数 `?key=调用密钥`。管理员可以在系统配置中显式开启无 Key 访问，此时请求按来源 IP 限流并以匿名请求记录。调用密钥由管理控制台创建。
- 管理控制台使用 `ADMIN_USERNAME`、`ADMIN_PASSWORD` 和一次性图形验证码登录，并使用 8 小时的 `HttpOnly`、`Secure`、`SameSite=Strict` Cookie 会话。
- 管理员密码、验证码答案和会话明文都不会写入 SQLite、审计日志或浏览器存储。
- 调用密钥、管理员会话和验证码答案只保存 SHA-256 哈希；新调用密钥只会在创建时显示一次。
- Cursor Token 支持多凭证池，并使用独立密钥进行 AES-GCM 加密后保存；控制台和接口只返回掩码。
- 同一调用 API Key 默认粘到同一个 Cursor Token；凭证认证失败或被限流时，服务会冷却该 Token 并自动切换一次。
- 可选 Redis 用于多实例共享限流、Token 粘性、健康状态和进行中请求计数；Redis 不可用时自动降级到单实例内存协调。
- 审计日志默认保存 30 天，只记录请求元数据，不记录请求体、响应体、Cursor Token 或任意 API Key。
- 服务默认绑定 `127.0.0.1:8041`。公网部署必须在 Caddy、Nginx 等 HTTPS 反向代理之后运行。

## 配置

创建 `config.yaml`：

```yaml
token: "你的 Cursor access token"
```

创建项目根目录的 `.env`（启动时自动加载，系统环境变量中的非空值优先）：

```dotenv
ADMIN_USERNAME=admin
ADMIN_PASSWORD=请使用高强度密码
```

然后从项目根目录启动：

```powershell
go run .
```

构建 Linux `amd64` 纯 Go 静态部署包（产物只包含一个可执行文件）：

```powershell
powershell -ExecutionPolicy Bypass -File .\build-linux.ps1
```

产物位于 `release/`：`cursor-tab-server-linux-amd64` 和对应的 `cursor-tab-server-linux-amd64.tar.gz`。运行时仍需要挂载或创建 `config.yaml`，并通过环境变量提供管理员凭据；数据库目录与 `token.key` 用于持久化运行数据和加密凭证。

可选环境变量：

- `LISTEN_ADDR`：监听地址，默认 `127.0.0.1:8041`。
- `DATABASE_PATH`：SQLite 文件，默认 `./data/cursor-tab-server.db`。
- `PROXY_RATE_PER_MINUTE`：调用 API Key + 来源 IP 的每分钟请求数，默认 `120`。
- `ADMIN_RATE_PER_MINUTE`：已登录管理接口的每来源 IP 每分钟请求数，默认 `30`。
- 管理控制台“系统配置”可选择是否允许无 API Key 访问代理；默认关闭。开启后请求按来源 IP 限流，并以匿名请求写入审计日志。
- 创建 API Key 时名称可留空，系统会自动生成名称。
- `REDIS_URL`：可选 Redis 连接地址，例如 `redis://127.0.0.1:6379/0`；未配置或连接失败时使用本机内存协调。
- `REDIS_PREFIX`：Redis 键前缀，默认 `cursor-tab`；同一集群中的所有服务实例必须一致。
- `TOKEN_KEY_PATH`：Cursor Token 的 AES-GCM 加密密钥文件，默认与数据库同目录下的 `token.key`。

首次启动会把 `config.yaml` 中的 Token 导入加密凭证池。之后可在控制台的“Token 池”页面添加和停用凭证；`config.yaml` 中的 Token 不会覆盖已经存在的池。请把 SQLite 数据库与 `token.key` 一起备份，丢失密钥后已加密的 Token 将无法恢复。

打开 `https://你的域名/`，填写管理员用户名、密码和图形验证码；在“API 密钥”页面创建供客户端调用代理接口的密钥。

## 调用代理

```bash
# 推荐：通过请求头传递密钥
curl https://你的域名/aiserver.v1.AiService/CppConfig \
  -H "X-API-Key: 创建后显示的调用密钥" \
  -H "Content-Type: application/json" \
  --data '{}'

# 兼容不方便设置请求头的客户端（推荐用于 Cursor 的“TAB service address”）
# 基础地址可用 /cts_... 或 /key=cts_... 两种形式；Cursor 会自动附加接口路径。
https://你的域名/key=创建后显示的调用密钥

# 若客户端支持自定义请求 URL，也可以使用查询参数
curl 'https://你的域名/aiserver.v1.AiService/CppConfig?key=创建后显示的调用密钥' \
  -H "Content-Type: application/json" \
  --data '{}'
```

查询参数中的 `key` 只用于本服务的鉴权，在转发给 Cursor 前会被移除。由于 URL 可能被浏览器历史、代理或访问日志记录，生产环境优先使用 `X-API-Key` 请求头。

未知代理路径返回 `404`，无效或停用的调用密钥返回 `401`，超过限流返回 `429`。

## 容器部署

构建镜像：

```bash
docker build -t cursor-tab-server .
```

`config.yaml` 不会写入镜像。以只读配置挂载、持久化数据库目录和运行时管理员账号启动：

```bash
docker run -d --name cursor-tab-server \
  -e ADMIN_USERNAME='admin' \
  -e ADMIN_PASSWORD='请使用高强度密码' \
  -e LISTEN_ADDR='0.0.0.0:8041' \
  -v /secure/cursor-tab-server/config.yaml:/app/config.yaml:ro \
  -v /srv/cursor-tab-server/data:/app/data \
  -p 127.0.0.1:8041:8041 \
  cursor-tab-server
```

将 Caddy 或 Nginx 配置为只通过 HTTPS 将公网流量代理至 `127.0.0.1:8041`。因为管理员 Cookie 使用 `Secure` 标记，生产环境不能使用明文 HTTP。定期备份持久化的 SQLite `data/` 目录，并限制其读取权限。

## 获取 Cursor Token

macOS：

```bash
sqlite3 "$HOME/Library/Application Support/Cursor/User/globalStorage/state.vscdb" \
  "SELECT value FROM ItemTable WHERE key = 'cursorAuth/accessToken';"
```

Windows：

```powershell
sqlite3 "$env:APPDATA\Cursor\User\globalStorage\state.vscdb" "SELECT value FROM ItemTable WHERE key = 'cursorAuth/accessToken';"
```
