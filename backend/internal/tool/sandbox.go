// Docker 沙箱。
//
// 隔离策略：一次性容器 + 无网络 + 资源限额 + 超时强杀 + 输出截断；
// 代码原文只存 sha256（sandbox_executions 表）。
// 沙箱是 Agent 安全的最后一道防线，没有隔离的 code_exec 不能上生产。
package tool

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// SandboxConfig 沙箱配置。
type SandboxConfig struct {
	Image     string        // agent/sandbox:latest
	Memory    string        // 512m
	CPUs      string        // "1"
	Timeout   time.Duration // 30s
	PoolSize  int
}

// Sandbox Docker 沙箱执行器。
type Sandbox struct {
	cli *client.Client
	cfg SandboxConfig
}

// NewSandbox 连接 Docker（读 DOCKER_HOST 环境变量，默认 /var/run/docker.sock）。
func NewSandbox(cfg SandboxConfig) (*Sandbox, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("new docker client: %w", err)
	}
	return &Sandbox{cli: cli, cfg: cfg}, nil
}

// ExecResult 沙箱执行结果。
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Run 在一次性容器中执行代码。
// language 决定解释器：python3 / go run / sh。
func (s *Sandbox) Run(ctx context.Context, language, code string) (*ExecResult, error) {
	start := time.Now()

	// 组装启动命令：每种语言统一为 [解释器, 文件] 或 [sh, -c, code]
	var cmd []string
	switch language {
	case "python":
		cmd = []string{"python3", "-c", code}
	case "shell":
		cmd = []string{"sh", "-c", code}
	case "go":
		// go run 单文件：代码需含 package main
		cmd = []string{"sh", "-c", fmt.Sprintf("mkdir -p /tmp/x && cat > /tmp/x/main.go <<'AGENTEOF'\n%s\nAGENTEOF\ncd /tmp/x && go run main.go", code)}
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	// 创建一次性容器：无网络 + 内存/CPU/PID 限额
	runCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	resp, err := s.cli.ContainerCreate(runCtx,
		&container.Config{
			Image:      s.cfg.Image,
			Cmd:        cmd,
			WorkingDir: "/sandbox",
			Tty:        false,
			Env:        []string{"GOCACHE=/tmp/gocache", "GOFLAGS=-mod=mod"},
		},
		&container.HostConfig{
			NetworkMode: "none",               // 无网络：最关键的隔离手段
			Resources: container.Resources{
				Memory:    512 * 1024 * 1024, // 512m
				NanoCPUs:  1e9,               // 1 CPU
				PidsLimit: int64Ptr(64),      // 防 fork 炸弹
			},
		},
		nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("container create: %w", err)
	}
	containerID := resp.ID
	// 无论成败，强制清理
	defer func() {
		rmCtx, rmCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer rmCancel()
		_ = s.cli.ContainerRemove(rmCtx, containerID, container.RemoveOptions{Force: true})
	}()

	if err := s.cli.ContainerStart(runCtx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("container start: %w", err)
	}

	// 等待退出（超时由 runCtx 强杀）
	statusCh, errCh := s.cli.ContainerWait(runCtx, containerID, container.WaitConditionNotRunning)
	select {
	case <-statusCh:
	case err := <-errCh:
		return nil, fmt.Errorf("container wait: %w", err)
	case <-runCtx.Done():
		return nil, fmt.Errorf("sandbox timeout (%s)", s.cfg.Timeout)
	}

	// 拉取日志
	logs, err := s.cli.ContainerLogs(runCtx, containerID, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: false,
	})
	if err != nil {
		return nil, fmt.Errorf("container logs: %w", err)
	}
	defer logs.Close()

	var stdout, stderr bytes.Buffer
	// TTY=false 时 stdout/stderr 分流
	_, _ = stdcopy.StdCopy(&stdout, &stderr, logs)

	inspect, err := s.cli.ContainerInspect(context.Background(), containerID)
	exitCode := -1
	if err == nil && inspect.State != nil {
		exitCode = inspect.State.ExitCode
	}

	return &ExecResult{
		Stdout:   truncate(strings.TrimSpace(stdout.String()), 64*1024),
		Stderr:   truncate(strings.TrimSpace(stderr.String()), 64*1024),
		ExitCode: exitCode,
		Duration: time.Since(start),
	}, nil
}

func int64Ptr(v int64) *int64 { return &v }

// truncate 截断输出。
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}
