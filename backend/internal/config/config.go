// Package config 配置加载：Viper 读取 configs/ + 环境变量 AGENT_ 前缀覆盖。
// 优先级：环境变量 AGENT_ > config.{env}.yaml > config.yaml
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 全局配置结构（字段与 configs/config.yaml 一一对应）。
type Config struct {
	Env           string        `mapstructure:"env"`
	HTTP          HTTP          `mapstructure:"http"`
	Log           Log           `mapstructure:"log"`
	PG            PG            `mapstructure:"pg"`
	Redis         Redis         `mapstructure:"redis"`
	Milvus        Milvus        `mapstructure:"milvus"`
	MinIO         MinIO         `mapstructure:"minio"`
	JWT           JWT           `mapstructure:"jwt"`
	SecretKey     string        `mapstructure:"secret_key"`
	LLM           LLM           `mapstructure:"llm"`
	RAG           RAG           `mapstructure:"rag"`
	Agent         Agent         `mapstructure:"agent"`
	Worker        Worker        `mapstructure:"worker"`
	Sandbox       Sandbox       `mapstructure:"sandbox"`
	Observability Observability `mapstructure:"observability"`
	MCP           MCP           `mapstructure:"mcp"`
	A2A           A2A           `mapstructure:"a2a"`
	Search        Search        `mapstructure:"search"`
}

type HTTP struct {
	Port            string        `mapstructure:"port"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type Log struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type PG struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type Milvus struct {
	Mode     string `mapstructure:"mode"` // remote / lite
	Addr     string `mapstructure:"addr"`
	LitePath string `mapstructure:"lite_path"`
	Dim      int    `mapstructure:"dim"`
}

type MinIO struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

type JWT struct {
	Secret     string        `mapstructure:"secret"`
	AccessTTL  time.Duration `mapstructure:"access_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
}

type LLM struct {
	MaxConnsPerProvider     int           `mapstructure:"max_conns_per_provider"`
	CircuitErrorRate        float64       `mapstructure:"circuit_error_rate"` //!熔断错误率阈值，如 0.3 = 30%
	CircuitOpen             time.Duration `mapstructure:"circuit_open"`       //!熔断打开后的冷却时长，如 60s
	RetryMax                int           `mapstructure:"retry_max"`
	StreamFirstTokenTimeout time.Duration `mapstructure:"stream_first_token_timeout"`
	Local                   bool          `mapstructure:"local"`             // 本地模式：全走 Ollama
	DefaultChat             string        `mapstructure:"default_chat"`      //!默认对话模型名
	DefaultEmbedding        string        `mapstructure:"default_embedding"` //!默认向量模型名
}

type RAG struct {
	ChunkSize      int     `mapstructure:"chunk_size"`
	ChunkOverlap   int     `mapstructure:"chunk_overlap"`
	DenseTopK      int32   `mapstructure:"dense_top_k"`
	SparseTopK     int32   `mapstructure:"sparse_top_k"`
	FusedTopN      int32   `mapstructure:"fused_top_n"`
	FinalTopK      int32   `mapstructure:"final_top_k"`
	RRFK           float64 `mapstructure:"rrf_k"`
	ScoreThreshold float32 `mapstructure:"score_threshold"`
	BatchEmbed     int     `mapstructure:"batch_embed"`
}

type Agent struct {
	MaxIterations      int           `mapstructure:"max_iterations"`
	StepTimeout        time.Duration `mapstructure:"step_timeout"`
	RunTimeout         time.Duration `mapstructure:"run_timeout"`
	MemoryExtractEvery int           `mapstructure:"memory_extract_every"`
}

type Worker struct {
	Concurrency   int           `mapstructure:"concurrency"`
	MaxRetry      int           `mapstructure:"max_retry"`
	ClaimInterval time.Duration `mapstructure:"claim_interval"`
}

type Sandbox struct {
	Enabled  bool          `mapstructure:"enabled"`
	Image    string        `mapstructure:"image"`
	Memory   string        `mapstructure:"memory"`
	CPUs     string        `mapstructure:"cpus"`
	Timeout  time.Duration `mapstructure:"timeout"`
	PoolSize int           `mapstructure:"pool_size"`
}

type Observability struct {
	OtelEndpoint   string `mapstructure:"otel_endpoint"`
	OtelEnabled    bool   `mapstructure:"otel_enabled"`
	MetricsEnabled bool   `mapstructure:"metrics_enabled"`
}

type MCP struct {
	Enabled        bool          `mapstructure:"enabled"`
	HealthInterval time.Duration `mapstructure:"health_interval"`
}

// A2A 对外暴露端点配置（本平台作为被调用方）。
type A2A struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    string `mapstructure:"port"` // 独立端口（net/http，与 Hertz 分离）
	Name    string `mapstructure:"name"`
	Desc    string `mapstructure:"desc"`
}

type Search struct {
	SearxngURL string `mapstructure:"searxng_url"`
}

// Load 加载配置。configDir 为空时使用 ./configs 与可执行文件旁的 configs/。
func Load(configDir string) (*Config, error) {
	if configDir == "" {
		// 依次尝试：工作目录/configs、可执行文件旁/configs
		for _, dir := range []string{"configs", filepath.Join(exeDir(), "configs")} {
			if isDir(dir) {
				configDir = dir
				break
			}
		}
		if configDir == "" {
			configDir = "configs"
		}
	}

	env := os.Getenv("AGENT_ENV")
	if env == "" {
		env = "dev"
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config.yaml: %w", err)
	}
	// 环境专属配置（可选）
	v.SetConfigName("config." + env)
	if err := v.MergeInConfig(); err != nil {
		// 不存在则忽略
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("merge config.%s.yaml: %w", env, err)
		}
	}

	// 环境变量覆盖：http.port -> AGENT_HTTP__PORT
	v.SetEnvPrefix("AGENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()

	// 环境变量覆盖 duration 等字段时 viper 不做类型转换，这里对关键项手动兜底
	if s := os.Getenv("AGENT_HTTP__PORT"); s != "" {
		v.Set("http.port", s)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	cfg.Env = env
	return &cfg, nil
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
