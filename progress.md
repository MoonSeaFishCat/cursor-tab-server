# Progress

## 2026-08-26

- Analyzed the existing Go reverse proxy and recorded security gaps.
- Confirmed implementation scope: key authentication for proxy users, administrator controls, SQLite audit storage, 30-day retention and rate limits.
- Created and reviewed the initial backend design specification.
- Confirmed React + Vite + shadcn/ui frontend, Go-embedded production build and an 8-hour HttpOnly administrator session.
- Collected visual choices using the design companion: fixed sidebar with neutral, spacious styling.
- Implemented Go configuration, SQLite migrations and repositories for hashed calling keys, administrator sessions and metadata-only audit records.
- Added separate calling-key and administrator-session authentication, 30-day audit cleanup, per-subject in-memory rate limiting, protected proxy routing and JSON management endpoints.
- Added the React/Vite management console with a neutral fixed sidebar, login, API-key creation/disable, audit logs, service status and logout; production assets are embedded in the Go service after the frontend build.
- Built a multi-stage Node + Go container image and verified `/healthz` from a container when the container binds `0.0.0.0:8041` internally while the host publishes only `127.0.0.1`.
- Verification passed: `go test ./...`, `go vet ./...`, `npm --prefix web test -- --run`, `npm --prefix web run build`, `npm --prefix web audit --omit=dev --audit-level=high`, and `docker build -t cursor-tab-server:test .`.
- 用户确认将管理员入口改为 `ADMIN_USERNAME` + `ADMIN_PASSWORD` 与服务端随机图形验证码。
- 已实现随机 6 位、可读字符的图形验证码、SQLite 哈希存储与单次使用、过期记录清理、获取及登录来源 IP 限流、账户密码登录和安全 Cookie 会话。
- 已将 React 登录页替换为用户名、密码、验证码图片、刷新验证码和统一认证错误提示；敏感登录数据不写入浏览器存储。
- 已将启动入口移至项目根目录 `main.go`，并更新 Docker 构建和部署文档。
- 验证通过：`go test ./...`、`go vet ./...`、`go build .`、`npm test -- --run`、`npm run build`、`docker build -t cursor-tab-server:test .`。
