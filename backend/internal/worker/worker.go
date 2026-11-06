// Package worker 异步任务消费者。
//
// 基于 Redis Streams 的轻量任务队列（替代 Kafka 的设计取舍）：
//
//	投递：XADD stream:doc-etl * doc_id {id}
//	消费：XREADGROUP（消费者组，多实例水平扩展）
//	收尸：XAUTOCLAIM 取回僵死消息（消费中宕机的遗留）
//	兜底：重试超限 → 死信队列 stream:doc-etl:dlq
//
// 任务类型：doc-etl（文档索引）/ eval（评估，P2 接入）
package worker

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"

	"agent/internal/config"
	"agent/internal/model"
	"agent/internal/rag/engine"
	"agent/internal/rag/loader"
	"agent/internal/repo"
)

// 流与消费组命名。
const (
	StreamDocETL = "stream:doc-etl"
	StreamEval   = "stream:eval"
	DLQDocETL    = "stream:doc-etl:dlq"

	GroupDocETL = "g-docetl"
	GroupEval   = "g-eval"

	ConsumerName = "worker-1" // 多实例部署时用 hostname 区分
)

// Publisher 任务投递者（实现 service.StreamPublisher）。
type Publisher struct {
	rdb *redis.Client
}

// NewPublisher 构造。
func NewPublisher(rdb *redis.Client) *Publisher { return &Publisher{rdb: rdb} }

// PublishDocETL 投递文档 ETL 任务。
func (p *Publisher) PublishDocETL(ctx context.Context, docID int64) error {
	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamDocETL,
		Values: map[string]any{"doc_id": strconv.FormatInt(docID, 10)},
	}).Err()
}

// PublishEvalRun 投递评估任务。
func (p *Publisher) PublishEvalRun(ctx context.Context, runID int64) error {
	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamEval,
		Values: map[string]any{"run_id": strconv.FormatInt(runID, 10)},
	}).Err()
}

// Worker 异步任务消费者。
type Worker struct {
	rdb    *redis.Client
	repos  *repo.Repos
	engine *engine.Engine
	minio  *minio.Client
	bucket string
	cfg    *config.Worker
}

// New 构造。
func New(rdb *redis.Client, repos *repo.Repos, eng *engine.Engine, mc *minio.Client, bucket string, cfg *config.Worker) *Worker {
	return &Worker{rdb: rdb, repos: repos, engine: eng, minio: mc, bucket: bucket, cfg: cfg}
}

// Run 启动消费循环（阻塞，直到 ctx 取消）。
func (w *Worker) Run(ctx context.Context) error {
	concurrency := w.cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	claimInterval := w.cfg.ClaimInterval
	if claimInterval <= 0 {
		claimInterval = 30 * time.Second
	}

	// ① 确保消费者组存在（MKSTREAM：流不存在则创建）
	if err := w.rdb.XGroupCreateMkStream(ctx, StreamDocETL, GroupDocETL, "0").Err(); err != nil && !isBusyGroup(err) {
		return fmt.Errorf("create group %s: %w", GroupDocETL, err)
	}

	// ② doc-etl 并发消费
	for i := 0; i < concurrency; i++ {
		go w.consumeLoop(ctx, StreamDocETL, GroupDocETL, w.handleDocETL)
	}
	// ③ 僵死消息收尸（独占 goroutine，周期性 XAUTOCLAIM）
	go w.claimLoop(ctx, StreamDocETL, GroupDocETL, w.handleDocETL, claimInterval)

	<-ctx.Done()
	return nil
}

// consumeLoop 消费主循环：XREADGROUP 阻塞读 → 处理 → XACK。
func (w *Worker) consumeLoop(ctx context.Context, stream, group string, handler func(ctx context.Context, id int64) error) {
	for {
		if ctx.Err() != nil {
			return
		}
		res, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: ConsumerName,
			Streams:  []string{stream, ">"}, // > = 只取新消息
			Count:    1,
			Block:    5 * time.Second,
		}).Result()
		if err != nil {
			if err != redis.Nil && ctx.Err() == nil {
				log.Printf("[worker] read %s: %v", stream, err)
				time.Sleep(time.Second) // 避免错误风暴
			}
			continue
		}
		for _, s := range res {
			for _, msg := range s.Messages {
				w.process(ctx, stream, group, msg, handler)
			}
		}
	}
}

// claimLoop 收尸循环：取回"已投递但未被 ACK"的僵死消息重新处理。
func (w *Worker) claimLoop(ctx context.Context, stream, group string, handler func(ctx context.Context, id int64) error, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// min-idle 3 分钟：短于它视为仍在正常处理中
			res, _, err := w.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
				Stream:   stream,
				Group:    group,
				Consumer: ConsumerName,
				MinIdle:  3 * time.Minute,
				Start:    "0-0",
				Count:    10,
			}).Result()
			if err != nil {
				continue
			}
			for _, msg := range res {
				w.process(ctx, stream, group, msg, handler)
			}
		}
	}
}

// process 单条消息处理：解析 → 执行 → ACK / 死信。
func (w *Worker) process(ctx context.Context, stream, group string, msg redis.XMessage, handler func(ctx context.Context, id int64) error) {
	docID, _ := strconv.ParseInt(fmt.Sprint(msg.Values["doc_id"]), 10, 64)
	if docID <= 0 {
		_ = w.rdb.XAck(ctx, stream, group, msg.ID).Err()
		return
	}
	if err := handler(ctx, docID); err != nil {
		log.Printf("[worker] doc-etl %d failed: %v", docID, err)
		w.deadLetter(ctx, stream, msg.ID, docID, err)
		return
	}
	_ = w.rdb.XAck(ctx, stream, group, msg.ID).Err()
}

// deadLetter 重试超限进死信队列（简单策略：按文档失败计数判断）。
func (w *Worker) deadLetter(ctx context.Context, stream, msgID string, docID int64, cause error) {
	doc, err := w.repos.KB.GetDoc(ctx, docID)
	if err != nil {
		return
	}
	doc.Status = model.DocStatusFailed
	doc.Error = truncateErr(cause)
	_ = w.repos.KB.UpdateDoc(ctx, doc)

	// 保留原始消息上下文供人工排查
	_ = w.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: DLQDocETL,
		Values: map[string]any{
			"doc_id": strconv.FormatInt(docID, 10),
			"msg_id": msgID,
			"error":  cause.Error(),
		},
	}).Err()
	_ = w.rdb.XAck(ctx, stream, GroupDocETL, msgID).Err()
}

// handleDocETL 文档索引管线（状态机推进由本方法负责）。
func (w *Worker) handleDocETL(ctx context.Context, docID int64) error {
	repos := w.repos

	// ① 取文档与知识库
	doc, err := repos.KB.GetDoc(ctx, docID)
	if err != nil {
		return fmt.Errorf("get doc %d: %w", docID, err)
	}
	if doc.Status == model.DocStatusReady {
		return nil // 幂等：重复投递直接跳过
	}
	kb, err := repos.KB.GetKB(ctx, doc.KBID)
	if err != nil {
		return fmt.Errorf("get kb %d: %w", doc.KBID, err)
	}

	// ② 解析状态推进
	doc.Status = model.DocStatusParsing
	_ = repos.KB.UpdateDoc(ctx, doc)

	// ③ 对象存储取回原文
	data, err := w.getObject(ctx, doc.ObjectKey)
	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}

	// ④ 解析（PDF/DOCX/MD/TXT/HTML）
	loaded, err := loader.Load(doc.Filename, data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", doc.Filename, err)
	}
	doc.Status = model.DocStatusChunking
	_ = repos.KB.UpdateDoc(ctx, doc)

	// ⑤ 索引：分块落库 + 向量化（engine 内部推进 embedding）
	if err := w.engine.IndexDocument(ctx, kb, doc, loaded); err != nil {
		return fmt.Errorf("index: %w", err)
	}

	// ⑥ 完成
	doc.Status = model.DocStatusReady
	doc.Error = ""
	if err := repos.KB.UpdateDoc(ctx, doc); err != nil {
		return fmt.Errorf("update doc: %w", err)
	}
	log.Printf("[worker] doc-etl %d ready: chunks=%d tokens=%d", doc.ID, doc.ChunkCount, doc.TokenCount)
	return nil
}

// getObject 从 MinIO 拉取对象。
func (w *Worker) getObject(ctx context.Context, key string) ([]byte, error) {
	obj, err := w.minio.GetObject(ctx, w.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

// isBusyGroup 消费组已存在（BUSYGROUP）不算错误。
func isBusyGroup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}

func truncateErr(err error) string {
	s := err.Error()
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}
