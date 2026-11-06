// 内置工具：文件操作与数据库只读查询。
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

// FileOpsConfig file_ops 配置。
type FileOpsConfig struct {
	// BaseDir 授权根目录（所有操作限制在该目录内，防路径穿越）
	BaseDir string
	MaxSize int
}

// RegisterFiles 注册 file_ops。
func RegisterFiles(r Registry, cfg FileOpsConfig) {
	r.Register("file_ops", func(ctx context.Context, args map[string]any, _ string) (string, error) {
		return runFileOps(cfg, args)
	})
}

// runFileOps 读/保存文件（路径限制在 BaseDir 内）。
func runFileOps(cfg FileOpsConfig, args map[string]any) (string, error) {
	operation, _ := args["operation"].(string)
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// 路径安全：拼出绝对路径后必须仍在 BaseDir 下
	clean := filepath.Clean(filepath.Join(cfg.BaseDir, path))
	if !strings.HasPrefix(clean, filepath.Clean(cfg.BaseDir)) {
		return "", fmt.Errorf("path escapes authorized directory")
	}

	switch operation {
	case "read":
		data, err := os.ReadFile(clean)
		if err != nil {
			return "", fmt.Errorf("read file: %v", err)
		}
		if len(data) > cfg.MaxSize {
			data = data[:cfg.MaxSize]
		}
		return string(data), nil
	case "save":
		content, _ := args["content"].(string)
		if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(clean, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write file: %v", err)
		}
		return fmt.Sprintf(`{"saved": %q, "bytes": %d}`, path, len(content)), nil
	default:
		return "", fmt.Errorf("operation must be read/save")
	}
}

// RegisterDBQuery 注册 db_query（只读 SELECT，禁 DML/DDL）。
func RegisterDBQuery(r Registry, db *gorm.DB) {
	r.Register("db_query", func(ctx context.Context, args map[string]any, _ string) (string, error) {
		return runDBQuery(ctx, db, args)
	})
}

var dmlRe = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|truncate|create|grant|revoke|copy)\b`)

// runDBQuery 只读 SQL 查询。
// 安全三重防线：正则禁 DML → 只读数据库账号 → 语句级超时。
func runDBQuery(ctx context.Context, db *gorm.DB, args map[string]any) (string, error) {
	sqlText, _ := args["sql"].(string)
	if sqlText == "" {
		return "", fmt.Errorf("sql is required")
	}
	if dmlRe.MatchString(sqlText) {
		return "", fmt.Errorf("only read-only SELECT allowed")
	}

	qCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	rows, err := db.WithContext(qCtx).Raw(sqlText).Rows()
	if err != nil {
		return "", fmt.Errorf("query: %v", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	// 收集为 map 数组（JSON 输出，模型可读）
	out := make([]map[string]any, 0, 32)
	values := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return "", err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			v := values[i]
			// []byte → string（JSON 友好）
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			row[c] = v
		}
		out = append(out, row)
		if len(out) >= 100 { // 行数上限，防大结果集打爆上下文
			break
		}
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}
