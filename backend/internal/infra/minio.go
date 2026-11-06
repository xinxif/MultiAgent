// 基础设施：MinIO 对象存储（原始文档/附件）。
package infra

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"agent/internal/config"
)

// NewMinIO 初始化 MinIO 客户端并确保桶存在。
func NewMinIO(ctx context.Context, cfg *config.MinIO) (*minio.Client, error) {
	cli, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("new minio client: %w", err)
	}

	exists, err := cli.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := cli.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket %s: %w", cfg.Bucket, err)
		}
	}
	return cli, nil
}

// ObjectKey 生成文档对象 Key：docs/{kbID}/{docID}_{filename}。
func ObjectKey(kbID, docID int64, filename string) string {
	return fmt.Sprintf("docs/%d/%d_%s", kbID, docID, filename)
}
