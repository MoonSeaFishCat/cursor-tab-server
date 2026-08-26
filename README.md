# Cursor Tab Server

一个带调用 API Key、审计日志和 React 管理控制台的 Cursor 服务代理。它使用你自己的 Cursor 登录令牌调用固定白名单中的上游接口；它不会绕过 Cursor 的订阅、额度、计费或权限校验。

## 安全模型

- 所有代理请求必须带 `X-API-Key`；也可使用查询参数 `?key=调用密钥`。调用密钥由管理控制台创建。
- 管理控制台使用 `ADMIN_USERNAME`、`ADMIN_PASSWORD` 和一次性图形验证码登录，并使用 8 小时的 `HttpOnly`、`Secure`、`SameSite=Strict` Cookie 会话。
- 管理员密码、验证码答案和会话明文都不会写入 SQLite、审计日志或浏览器存储。
- 调用密钥、管理员会话和验证码答案只保存 SHA-256 哈希；新调用密钥只会在创建时显示一次。
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

可选环境变量：

- `LISTEN_ADDR`：监听地址，默认 `127.0.0.1:8041`。
- `DATABASE_PATH`：SQLite 文件，默认 `./data/cursor-tab-server.db`。
- `PROXY_RATE_PER_MINUTE`：调用 API Key + 来源 IP 的每分钟请求数，默认 `120`。
- `ADMIN_RATE_PER_MINUTE`：已登录管理接口的每来源 IP 每分钟请求数，默认 `30`。

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
