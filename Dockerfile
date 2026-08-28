FROM node:22-alpine AS web-build

WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25 AS go-build

WORKDIR /src
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=web-build /src/internal/httpapi/assets/ ./internal/httpapi/assets/
RUN go build -trimpath -tags "netgo osusergo" -ldflags="-s -w -extldflags=-static" -o /out/cursor-tab-server .

FROM scratch

WORKDIR /app
COPY --from=go-build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=go-build /out/cursor-tab-server /app/cursor-tab-server
EXPOSE 8041
ENTRYPOINT ["/app/cursor-tab-server"]
