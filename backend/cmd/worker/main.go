// Agent 异步任务 Worker 入口：独立进程消费 Redis Streams。
//
// 设计取舍：独立进程
//   - ETL/评估是 CPU/IO 密集任务，与 API 进程隔离避免互相拖累
//   - 独立伸缩：文档上传高峰时只扩 worker 副本
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"agent/internal/config"
	"agent/internal/infra"
	"agent/internal/llm"
	"agent/internal/observability"
        "agent/internal/observability/logger"
        "agent/internal/security"
	"agent/internal/rag/engine"
	"agent/internal/repo"
	"agent/internal/service"
	"agent/internal/vector"
	"agent/internal/worker"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "worker exited: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}
	logger.Init(cfg.Log.Level, cfg.Log.Format)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 基础设施（API 与 worker 共享同一套存储）
	db, err := infra.NewPG(ctx, &cfg.PG)
	if err != nil {
		return fmt.Errorf("pg: %w", err)
	}
	rdb, err := infra.NewRedis(ctx, &cfg.Redis)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	mcli, err := infra.NewMilvus(ctx, &cfg.Milvus)
	if err != nil {
		return fmt.Errorf("milvus: %w", err)
	}

	// RAG 引擎（doc-etl 消费方）
        gateway := llm.NewGateway(&cfg.LLM)
        store := vector.NewMilvusStore(mcli)
        repos := repo.New(db)

        // 加密器与 server 同源：模型解析需解密 api_key（缺失会 nil panic）
        encryptor, err := security.NewEncryptor(cfg.SecretKey)
        if err != nil {
                return fmt.Errorf("encryptor: %w", err)
        }

        // 默认模型解析与 server 同源（service.New 仅用于模型解析）
        svcs, err := service.New(service.Deps{
                Cfg: cfg, Repos: repos, Gateway: gateway, RDB: rdb, Vector: store, Encryptor: encryptor,
        })
	if err != nil {
		return fmt.Errorf("services: %w", err)
	}
	ragEngine := engine.New(gateway, store, repos.KB, &cfg.RAG, cfg.Milvus.Dim)
	ragEngine.SetModels(svcs.DefaultEmbedding, svcs.DefaultRerank)

	w := worker.New(rdb, repos, ragEngine, nil, "", &cfg.Worker)
	// MinIO 可选（未配置时关闭对象存储，文档内容直传）
	if mc, err := infra.NewMinIO(ctx, &cfg.MinIO); err == nil {
		w = worker.New(rdb, repos, ragEngine, mc, cfg.MinIO.Bucket, &cfg.Worker)
	}

	observability.LogInfo("agent worker started",
		"concurrency", fmt.Sprint(cfg.Worker.Concurrency), "stream", worker.StreamDocETL)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		cancel()
	}()
	return w.Run(ctx)
}
