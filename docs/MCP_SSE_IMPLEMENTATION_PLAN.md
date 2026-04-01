# MCP SSE 传输协议支持技术方案

> **文档状态**: ⚠️ 已弃用 - 已被 Streamable HTTP 替代  
> **实施日期**: 2026-04-01  
> **替代文档**: `MCP_STREAMABLE_HTTP_IMPLEMENTATION.md`

## 0. 重要说明

**本文档已弃用**。根据 MCP 规范（2025-03-26+），**SSE 传输已被 Streamable HTTP 替代**。

实际实施请参考：
- ✅ **实施文档**: `docs/MCP_STREAMABLE_HTTP_IMPLEMENTATION.md`
- ✅ **设计文档**: `docs/MCP_SERVER_DESIGN.md` (Section 12: Streamable HTTP 传输)

### 为什么弃用 SSE？

1. **MCP 规范变更**: 2025-03-26+ 版本正式弃用 SSE
2. **Streamable HTTP 优势**:
   - 单端点设计（`/mcp` vs SSE 的 `/sse` + `/message`）
   - 更简单的协议栈
   - 更好的现代 Web 兼容性
   - 完整的双向通信支持

### 本文档价值

本文档仍保留了传输协议选型的分析过程，可供参考：
- 传输协议对比分析（Section 1-2）
- 架构设计思路（Section 3）
- 安全考虑（Section 5）

---

## 1. 背景与目标

### 1.1 现状

当前 PageIndex Go MCP Server 仅支持 **stdio 传输协议**，适用于：
- Claude Desktop（本地进程）
- Cursor（本地进程）
- Cline（本地进程）

**局限性**：
- ❌ 无法远程部署（必须本地执行二进制文件）
- ❌ 无法多客户端并发连接（单进程绑定）
- ❌ 无法负载均衡/水平扩展
- ❌ 鉴权能力弱（依赖客户端控制）

### 1.2 SSE 优势

**Server-Sent Events (SSE)** 是 MCP 官方推荐的 HTTP 传输协议：

| 特性 | stdio | SSE (HTTP) |
|------|-------|-----------|
| **部署方式** | 本地进程 | 远程服务 |
| **连接数** | 单客户端 | 多客户端并发 |
| **网络访问** | 无 | 支持跨网络 |
| **鉴权** | 无 | Token/API Key/OAuth |
| **负载均衡** | 不支持 | 支持 |
| **服务发现** | 无 | 支持 |
| **监控可观测** | 困难 | 容易（标准 HTTP 指标） |

### 1.3 目标

为 PageIndex Go MCP Server 添加 **SSE 传输协议支持**，实现：
1. ✅ 支持 HTTP + SSE 远程访问
2. ✅ 支持多客户端并发连接
3. ✅ 支持 API Key 鉴权
4. ✅ 保留 stdio 支持（向后兼容）
5. ✅ 支持两种模式独立启动或同时运行

---

## 2. 技术架构

### 2.1 架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│  MCP Clients                                                    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ Claude Desktop│  │   Cursor     │  │    Cline     │          │
│  │  (stdio)     │  │  (stdio/SSE) │  │  (stdio/SSE) │          │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘          │
│         │                  │                  │                  │
│         │ stdio            │ HTTP+SSE         │ HTTP+SSE         │
└─────────┼──────────────────┼──────────────────┼──────────────────┘
          │                  │                  │
          ▼                  ▼                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  PageIndex Go MCP Server                                        │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │  cmd/mcp/main.go                                          │ │
│  │  - 支持 --transport stdio|sse|both                       │ │
│  │  - 支持 --addr :8080 (SSE 监听地址)                       │ │
│  │  - 支持 --auth-token xxx (可选鉴权)                       │ │
│  └───────────────────────────────────────────────────────────┘ │
│  ┌─────────────────────┐  ┌─────────────────────────────────┐ │
│  │  pkg/mcp/stdio.go   │  │  pkg/mcp/sse.go                 │ │
│  │  - StdioServer      │  │  - SSEServer                    │ │
│  │  - ServeStdin()     │  │  - NewSSEServer()               │ │
│  │                     │  │  - Start()                      │ │
│  │                     │  │  - Shutdown()                   │ │
│  └─────────────────────┘  └─────────────────────────────────┘ │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │  pkg/mcp/server.go (共用)                                 │ │
│  │  - NewMCPServer()  - 创建 MCPServer 实例                 │ │
│  │  - registerTools() - 注册工具                            │ │
│  │  - generateIndexHandler                                   │ │
│  │  - searchIndexHandler                                     │ │
│  │  - updateIndexHandler                                     │ │
│  └───────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  现有业务逻辑 (复用)                                            │
│  - pkg/indexer/                                                 │
│  - pkg/document/                                                │
│  - pkg/llm/                                                     │
│  - pkg/output/                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 端点设计

**SSE 模式需要两个 HTTP 端点**：

| 端点 | 方法 | 用途 | 示例 |
|------|------|------|------|
| `/sse` | GET | 建立 SSE 连接，服务端推送事件 | `GET /sse` → `event: endpoint\ndata: /message?sessionId=xxx` |
| `/message` | POST | 客户端发送 JSON-RPC 请求 | `POST /message?sessionId=xxx` |

**通信流程**：
```
Client                              Server
   │                                   │
   │──── GET /sse ───────────────────> │
   │                                   │
   │<─── event: endpoint ───────────── │
   │       data: /message?sid=uuid     │
   │                                   │
   │──── POST /message?sid=uuid ─────> │ (JSON-RPC Request)
   │                                   │
   │<─── event: message ────────────── │ (JSON-RPC Response via SSE)
   │                                   │
```

---

## 3. 实现方案

### 3.1 目录结构

```
pkg/mcp/
├── server.go          # 现有：MCPServer 创建和工具注册
├── tools.go           # 现有：工具处理器
├── types.go           # 现有：类型定义
├── stdio.go           # 现有：stdio 服务器（从 server.go 重构）
├── sse.go             # 新建：SSE 服务器实现
├── auth.go            # 新建：认证中间件（可选）
└── config.go          # 新建：配置结构

cmd/mcp/
└── main.go            # 修改：支持多传输模式
```

### 3.2 SSE 服务器实现 (`pkg/mcp/sse.go`)

```go
package mcp

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/mark3labs/mcp-go/server"
)

// SSEConfig 定义 SSE 服务器配置
type SSEConfig struct {
    Addr         string        // 监听地址，如 ":8080"
    BasePath     string        // 基础路径，如 "/mcp"
    SSEEndpoint  string        // SSE 端点，默认 "/sse"
    MessageEndpoint string     // Message 端点，默认 "/message"
    AuthToken    string        // API Token（可选）
    ReadTimeout  time.Duration // HTTP 读超时
    WriteTimeout time.Duration // HTTP 写超时
    IdleTimeout  time.Duration // HTTP 空闲超时
}

// SSEServer 封装 SSE MCP 服务器
type SSEServer struct {
    config  *SSEConfig
    mcpsrv  *server.MCPServer
    sseSrv  *server.SSEServer
    httpSrv *http.Server
}

// NewSSEServer 创建 SSE 服务器实例
func NewSSEServer(mcpsrv *server.MCPServer, cfg *SSEConfig) *SSEServer {
    // 创建 SSE 服务器
    sseSrv := server.NewSSEServer(
        mcpsrv,
        server.WithBaseURL(""),  // 不设置 baseURL，让客户端自行拼接
        server.WithStaticBasePath(cfg.BasePath),
        server.WithSSEEndpoint(cfg.SSEEndpoint),
        server.WithMessageEndpoint(cfg.MessageEndpoint),
        server.WithKeepAlive(true),
        server.WithKeepAliveInterval(30*time.Second),
    )

    // 创建 HTTP 服务器
    mux := http.NewServeMux()
    
    // 注册 SSE 和 Message 处理器
    mux.Handle(cfg.BasePath+cfg.SSEEndpoint, sseSrv.SSEHandler())
    mux.Handle(cfg.BasePath+cfg.MessageEndpoint, sseSrv.MessageHandler())
    
    // 添加认证中间件（如果配置了 AuthToken）
    var handler http.Handler = mux
    if cfg.AuthToken != "" {
        handler = withAuthTokenAuth(cfg.AuthToken, mux)
    }

    httpSrv := &http.Server{
        Addr:         cfg.Addr,
        Handler:      handler,
        ReadTimeout:  cfg.ReadTimeout,
        WriteTimeout: cfg.WriteTimeout,
        IdleTimeout:  cfg.IdleTimeout,
    }

    return &SSEServer{
        config:  cfg,
        mcpsrv:  mcpsrv,
        sseSrv:  sseSrv,
        httpSrv: httpSrv,
    }
}

// Start 启动 SSE 服务器
func (s *SSEServer) Start() error {
    fmt.Printf("🚀 PageIndex MCP Server (SSE) starting on %s\n", s.config.Addr)
    fmt.Printf("📡 SSE endpoint: http://%s%s%s\n", s.config.Addr, s.config.BasePath, s.config.SSEEndpoint)
    fmt.Printf("💬 Message endpoint: http://%s%s%s\n", s.config.Addr, s.config.BasePath, s.config.MessageEndpoint)
    
    if s.config.AuthToken != "" {
        fmt.Println("🔒 Authentication: Enabled (Bearer Token)")
    } else {
        fmt.Println("⚠️  Authentication: Disabled")
    }

    return s.httpSrv.ListenAndServe()
}

// Shutdown 优雅关闭 SSE 服务器
func (s *SSEServer) Shutdown(ctx context.Context) error {
    fmt.Println("🛑 Shutting down SSE server...")
    return s.httpSrv.Shutdown(ctx)
}

// GetEndpoints 返回服务器端点信息
func (s *SSEServer) GetEndpoints() (sseURL, messageURL string) {
    sseURL = fmt.Sprintf("http://%s%s%s", s.config.Addr, s.config.BasePath, s.config.SSEEndpoint)
    messageURL = fmt.Sprintf("http://%s%s%s", s.config.Addr, s.config.BasePath, s.config.MessageEndpoint)
    return
}

// withAuthTokenAuth 创建 Bearer Token 认证中间件
func withAuthTokenAuth(token string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
            return
        }

        // 支持 Bearer Token 格式
        expected := "Bearer " + token
        if authHeader != expected {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

### 3.3 重构 stdio 支持 (`pkg/mcp/stdio.go`)

从现有 `server.go` 中提取 stdio 逻辑：

```go
package mcp

import (
    "context"
    "fmt"

    "github.com/mark3labs/mcp-go/server"
)

// StdioServer 封装 Stdio MCP 服务器
type StdioServer struct {
    mcpsrv *server.MCPServer
}

// NewStdioServer 创建 Stdio 服务器实例
func NewStdioServer(mcpsrv *server.MCPServer) *StdioServer {
    return &StdioServer{
        mcpsrv: mcpsrv,
    }
}

// ServeStdin 开始处理 stdio 连接
func (s *StdioServer) ServeStdin() error {
    fmt.Println("🚀 PageIndex MCP Server (stdio) starting...")
    fmt.Println("📡 Waiting for connections on stdin/stdout...")

    stdioServer := server.NewStdioServer(s.mcpsrv)
    return stdioServer.ServeStdin()
}

// Shutdown 优雅关闭 Stdio 服务器
func (s *StdioServer) Shutdown(ctx context.Context) error {
    fmt.Println("🛑 Shutting down stdio server...")
    return nil // stdio 无需特殊清理
}
```

### 3.4 命令行入口 (`cmd/mcp/main.go`)

支持多传输模式：

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/xgsong/mypageindexgo/pkg/mcp"
)

type TransportMode string

const (
    ModeStdio TransportMode = "stdio"
    ModeSSE   TransportMode = "sse"
    ModeBoth  TransportMode = "both"
)

func main() {
    // 命令行参数
    var (
        transport   string
        addr        string
        basePath    string
        authToken   string
        showVersion bool
    )

    flag.StringVar(&transport, "transport", "stdio", "传输模式：stdio, sse, both")
    flag.StringVar(&addr, "addr", ":8080", "SSE 监听地址 (仅 SSE 模式)")
    flag.StringVar(&basePath, "base-path", "/", "HTTP 基础路径 (仅 SSE 模式)")
    flag.StringVar(&authToken, "auth-token", "", "API 认证 Token (可选)")
    flag.BoolVar(&showVersion, "version", false, "显示版本信息")
    flag.Parse()

    if showVersion {
        fmt.Println("PageIndex MCP Server v1.1.0")
        fmt.Println("Supported transports: stdio, SSE (HTTP)")
        os.Exit(0)
    }

    // 创建 MCP Server
    mcpsrv, err := mcp.NewMCPServer()
    if err != nil {
        fmt.Fprintf(os.Stderr, "❌ Failed to create MCP server: %v\n", err)
        os.Exit(1)
    }

    // 优雅关闭处理
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigChan
        fmt.Println("\n📶 Received shutdown signal")
        cancel()
    }()

    // 根据传输模式启动服务器
    mode := TransportMode(transport)
    
    switch mode {
    case ModeStdio:
        stdioSrv := mcp.NewStdioServer(mcpsrv)
        if err := stdioSrv.ServeStdin(); err != nil {
            fmt.Fprintf(os.Stderr, "❌ Stdio server error: %v\n", err)
            os.Exit(1)
        }

    case ModeSSE:
        sseCfg := &mcp.SSEConfig{
            Addr:         addr,
            BasePath:     basePath,
            SSEEndpoint:  "/sse",
            MessageEndpoint: "/message",
            AuthToken:    authToken,
        }
        sseSrv := mcp.NewSSEServer(mcpsrv, sseCfg)
        if err := sseSrv.Start(); err != nil {
            fmt.Fprintf(os.Stderr, "❌ SSE server error: %v\n", err)
            os.Exit(1)
        }

    case ModeBoth:
        // 同时启动 stdio 和 SSE
        stdioSrv := mcp.NewStdioServer(mcpsrv)
        sseCfg := &mcp.SSEConfig{
            Addr:         addr,
            BasePath:     basePath,
            SSEEndpoint:  "/sse",
            MessageEndpoint: "/message",
            AuthToken:    authToken,
        }
        sseSrv := mcp.NewSSEServer(mcpsrv, sseCfg)

        // 在 goroutine 中启动 SSE
        go func() {
            if err := sseSrv.Start(); err != nil {
                fmt.Fprintf(os.Stderr, "❌ SSE server error: %v\n", err)
                cancel()
            }
        }()

        // 主线程运行 stdio
        if err := stdioSrv.ServeStdin(); err != nil {
            fmt.Fprintf(os.Stderr, "❌ Stdio server error: %v\n", err)
            os.Exit(1)
        }

    default:
        fmt.Fprintf(os.Stderr, "❌ Invalid transport mode: %s\n", transport)
        flag.Usage()
        os.Exit(1)
    }

    // 等待关闭信号
    <-ctx.Done()
    fmt.Println("👋 Goodbye!")
}
```

### 3.5 认证中间件 (`pkg/mcp/auth.go`)

提供多种认证方式：

```go
package mcp

import (
    "crypto/subtle"
    "net/http"
    "strings"
)

// AuthConfig 认证配置
type AuthConfig struct {
    // Bearer Token 认证
    BearerToken string
    
    // API Key 认证（通过 X-API-Key header）
    APIKey string
    
    // 基本认证
    BasicAuth map[string]string // username -> password
}

// WithAuthentication 创建认证中间件
func WithAuthentication(cfg AuthConfig, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 跳过 OPTIONS 请求（CORS 预检）
        if r.Method == http.MethodOptions {
            next.ServeHTTP(w, r)
            return
        }

        // 按优先级尝试多种认证方式
        if cfg.BearerToken != "" {
            if checkBearerAuth(r, cfg.BearerToken) {
                next.ServeHTTP(w, r)
                return
            }
        }

        if cfg.APIKey != "" {
            if checkAPIKeyAuth(r, cfg.APIKey) {
                next.ServeHTTP(w, r)
                return
            }
        }

        if len(cfg.BasicAuth) > 0 {
            if checkBasicAuth(r, cfg.BasicAuth) {
                next.ServeHTTP(w, r)
                return
            }
        }

        // 如果配置了认证但未通过，返回 401
        if cfg.BearerToken != "" || cfg.APIKey != "" || len(cfg.BasicAuth) > 0 {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // 未配置认证，直接通过
        next.ServeHTTP(w, r)
    })
}

// checkBearerAuth 验证 Bearer Token
func checkBearerAuth(r *http.Request, expectedToken string) bool {
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return false
    }

    expected := "Bearer " + expectedToken
    return subtle.ConstantTimeCompare([]byte(authHeader), []byte(expected)) == 1
}

// checkAPIKeyAuth 验证 API Key
func checkAPIKeyAuth(r *http.Request, expectedKey string) bool {
    apiKey := r.Header.Get("X-API-Key")
    if apiKey == "" {
        return false
    }

    return subtle.ConstantTimeCompare([]byte(apiKey), []byte(expectedKey)) == 1
}

// checkBasicAuth 验证 Basic Auth
func checkBasicAuth(r *http.Request, credentials map[string]string) bool {
    username, password, ok := r.BasicAuth()
    if !ok {
        return false
    }

    expectedPassword, exists := credentials[username]
    if !exists {
        return false
    }

    return subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1
}

// CORS 中间件
func WithCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

---

## 4. 客户端配置示例

### 4.1 Claude Desktop (SSE 模式)

```json
{
  "mcpServers": {
    "pageindex": {
      "url": "http://localhost:8080/sse",
      "type": "sse",
      "headers": {
        "Authorization": "Bearer your-token-here"
      }
    }
  }
}
```

### 4.2 Cursor (SSE 模式)

`.cursor/mcp.json`:
```json
{
  "mcpServers": {
    "pageindex": {
      "url": "http://localhost:8080/sse",
      "type": "sse",
      "headers": {
        "Authorization": "Bearer your-token-here"
      }
    }
  }
}
```

### 4.3 自定义 HTTP Client

```typescript
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { SSEClientTransport } from "@modelcontextprotocol/sdk/client/sse.js";

const transport = new SSEClientTransport(
  new URL("http://localhost:8080/sse"),
  {
    eventSourceInit: {
      fetch: (url, init) => fetch(url, {
        ...init,
        headers: {
          ...init?.headers,
          "Authorization": "Bearer your-token-here"
        }
      })
    },
    requestInit: {
      headers: {
        "Authorization": "Bearer your-token-here"
      }
    }
  }
);

const client = new Client({
  name: "pageindex-client",
  version: "1.0.0",
}, {
  capabilities: {}
});

await client.connect(transport);
```

---

## 5. 部署方案

### 5.1 本地开发

```bash
# stdio 模式（默认，向后兼容）
./pageindex-mcp

# SSE 模式（本地测试）
./pageindex-mcp -transport sse -addr :8080

# SSE 模式 + 认证
./pageindex-mcp -transport sse -addr :8080 -auth-token "my-secret-token"

# 双模式同时运行
./pageindex-mcp -transport both -addr :8080 -auth-token "my-secret-token"
```

### 5.2 Docker 部署

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o pageindex-mcp ./cmd/mcp

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/pageindex-mcp .
COPY config.yaml .

EXPOSE 8080
CMD ["./pageindex-mcp", "-transport", "sse", "-addr", ":8080"]
```

```yaml
# docker-compose.yml
version: '3.8'
services:
  pageindex-mcp:
    build: .
    ports:
      - "8080:8080"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    volumes:
      - ./config.yaml:/root/config.yaml
      - ./data:/root/data
    restart: unless-stopped
```

### 5.3 Kubernetes 部署

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pageindex-mcp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: pageindex-mcp
  template:
    metadata:
      labels:
        app: pageindex-mcp
    spec:
      containers:
      - name: pageindex-mcp
        image: your-registry/pageindex-mcp:latest
        ports:
        - containerPort: 8080
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: pageindex-secrets
              key: openai-api-key
        args:
        - -transport
        - sse
        - -addr
        - :8080
        - -auth-token
        - $(BEARER_TOKEN)
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: pageindex-mcp
spec:
  selector:
    app: pageindex-mcp
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: pageindex-mcp
  annotations:
    nginx.ingress.kubernetes.io/auth-type: basic
    nginx.ingress.kubernetes.io/auth-secret: pageindex-auth
    nginx.ingress.kubernetes.io/auth-realm: "Authentication Required"
spec:
  rules:
  - host: mcp.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: pageindex-mcp
            port:
              number: 80
```

### 5.4 Nginx 反向代理

```nginx
server {
    listen 443 ssl;
    server_name mcp.yourdomain.com;

    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;

    # 支持 SSE (Server-Sent Events)
    proxy_buffering off;
    proxy_cache off;
    proxy_chunked_transfer_encoding off;
    
    # 重要：保持长连接
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Keep-Alive "timeout=300";
    
    # SSE 端点
    location /sse {
        proxy_pass http://localhost:8080/sse;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        
        # SSE 需要这些 header
        proxy_set_header Accept-Encoding gzip;
        proxy_set_header Content-Type "";
    }
    
    # Message 端点
    location /message {
        proxy_pass http://localhost:8080/message;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Authorization $http_authorization;
    }
}
```

---

## 6. 监控与可观测性

### 6.1 健康检查端点

```go
// pkg/mcp/sse.go 中添加
func (s *SSEServer) setupHealthEndpoint(mux *http.ServeMux) {
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"healthy"}`))
    })
    
    mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
        // 检查 LLM 客户端连接、配置等
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"ready"}`))
    })
}
```

### 6.2 Prometheus 指标

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    mcpConnectionsTotal = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "mcp_connections_total",
            Help: "Total number of MCP connections",
        },
    )
    
    mcpActiveConnections = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "mcp_active_connections",
            Help: "Current number of active MCP connections",
        },
    )
    
    mcpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "mcp_request_duration_seconds",
            Help:    "Request duration distribution",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method"},
    )
)

func RegisterMetrics() {
    prometheus.MustRegister(mcpConnectionsTotal)
    prometheus.MustRegister(mcpActiveConnections)
    prometheus.MustRegister(mcpRequestDuration)
}
```

### 6.3 结构化日志

```go
// 使用 zerolog 记录连接事件
log.Info().
    Str("session_id", sessionID).
    Str("remote_addr", r.RemoteAddr).
    Str("user_agent", r.UserAgent()).
    Msg("New SSE connection established")

log.Error().
    Str("session_id", sessionID).
    Err(err).
    Msg("SSE connection error")
```

---

## 7. 安全考虑

### 7.1 认证机制

| 方式 | 适用场景 | 安全性 |
|------|----------|--------|
| **Bearer Token** | 内部服务、受信任网络 | ⭐⭐⭐ |
| **API Key** | 服务间调用 | ⭐⭐⭐ |
| **OAuth 2.0** | 公共访问、第三方集成 | ⭐⭐⭐⭐⭐ |
| **mTLS** | 零信任网络 | ⭐⭐⭐⭐⭐ |

### 7.2 推荐配置

**生产环境最小安全配置**：
```bash
./pageindex-mcp \
  -transport sse \
  -addr :8080 \
  -auth-token "$(openssl rand -hex 32)" \
  -base-path /mcp
```

**通过 HTTPS**（必须）：
```bash
# Nginx 终止 SSL
./pageindex-mcp -transport sse -addr 127.0.0.1:8080 -auth-token "xxx"
```

**网络隔离**：
```yaml
# Kubernetes NetworkPolicy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: pageindex-mcp
spec:
  podSelector:
    matchLabels:
      app: pageindex-mcp
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: trusted-namespace
    ports:
    - protocol: TCP
      port: 8080
```

---

## 8. 实施计划

### Phase 1: 基础实现 (2-3 天)

- [ ] 重构 `pkg/mcp/server.go` 分离 stdio 逻辑
- [ ] 实现 `pkg/mcp/stdio.go`
- [ ] 实现 `pkg/mcp/sse.go`
- [ ] 更新 `cmd/mcp/main.go` 支持多模式
- [ ] 添加基础认证中间件

### Phase 2: 测试验证 (1-2 天)

- [ ] 编写 SSE 单元测试
- [ ] 编写集成测试
- [ ] 测试客户端兼容性（Claude Desktop, Cursor, Cline）
- [ ] 性能基准测试

### Phase 3: 文档与部署 (1 天)

- [ ] 更新 README.md
- [ ] 添加部署文档
- [ ] 创建 Dockerfile
- [ ] 创建 Kubernetes manifests
- [ ] 更新 MCP 设计文档

### Phase 4: 高级功能 (可选)

- [ ] OAuth 2.0 支持
- [ ] 速率限制
- [ ] 请求日志审计
- [ ] WebSocket 支持（双向通信）
- [ ] 连接池管理

---

## 9. 技术风险与缓解

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| **SSE 连接断开** | 中 | 中 | 实现自动重连、心跳检测 |
| **内存泄漏** | 高 | 低 | 连接超时、资源清理测试 |
| **并发连接过多** | 中 | 中 | 连接数限制、负载均衡 |
| **认证绕过** | 高 | 低 | 安全审计、渗透测试 |
| **Docker 网络问题** | 低 | 低 | 详细文档、网络诊断工具 |

---

## 10. 性能基准

### 预期性能指标

| 指标 | stdio | SSE (目标) |
|------|-------|-----------|
| **连接延迟** | <1ms | <10ms |
| **请求延迟** | <100ms | <150ms |
| **并发连接** | 1 | 100+ |
| **吞吐量** | 10 req/s | 1000 req/s |
| **内存占用** | 50MB | 100MB (100 connections) |

### 基准测试命令

```bash
# 使用 wrk 进行压力测试
wrk -t12 -c400 -d30s http://localhost:8080/message

# 使用 k6 进行 SSE 测试
k6 run sse_benchmark.js
```

---

## 11. 总结

### 方案优势

✅ **完全兼容 MCP 协议规范**  
✅ **复用现有 mark3labs/mcp-go SDK**（无需重复造轮子）  
✅ **向后兼容 stdio 模式**  
✅ **支持多种部署场景**（本地、Docker、K8s）  
✅ **生产级认证和安全性**  
✅ **易于监控和维护**

### 建议实施顺序

1. **立即实施**: Phase 1-3（基础 SSE 支持）
2. **短期优化**: 监控和日志（Phase 3）
3. **长期规划**: OAuth 2.0、WebSocket（Phase 4）

###  estimated 工作量

- **开发**: 3-4 天
- **测试**: 1-2 天
- **文档**: 0.5 天
- **总计**: 5-6.5 天

---

## 附录 A: 参考资料

1. [MCP Specification](https://spec.modelcontextprotocol.io/)
2. [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)
3. [SSE MDN Documentation](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
4. [Go HTTP Package](https://pkg.go.dev/net/http)

## 附录 B: 术语表

| 术语 | 定义 |
|------|------|
| **MCP** | Model Context Protocol - AI 模型上下文协议 |
| **SSE** | Server-Sent Events - 服务端推送技术 |
| **JSON-RPC** | 基于 JSON 的远程过程调用协议 |
| **Bearer Token** | HTTP 认证 token 格式 |
