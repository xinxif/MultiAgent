// 数据库迁移工具：执行 migrations/*.sql 并种子管理员账号。
//
// 为什么独立成 cmd：服务进程保持无状态启动（main.go 不做 DDL），
// 迁移由 `make migrate`（本命令）显式执行 —— 便于在 CI 中卡点执行。
//
// 用法：
//
//	go run ./cmd/migrate          # 执行全部 up 迁移 + 种子
//	go run ./cmd/migrate -down 1  # 回滚最近 1 个版本（危险，仅限开发环境）
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // lib/pq 驱动 + schema_migrations 表
	_ "github.com/golang-migrate/migrate/v4/source/file"       // file:// 读取 migrations/ 目录
	"golang.org/x/crypto/bcrypt"

	"agent/internal/config"
	"agent/internal/infra"
)

func main() {
	down := flag.Int("down", 0, "回滚 N 个版本（默认 0 = 只做升级）")
	flag.Parse()

	if err := run(*down); err != nil {
		fmt.Fprintf(os.Stderr, "migrate failed: %v\n", err)
		os.Exit(1)
	}
}

func run(down int) error {
	// ① 复用统一配置加载（PG DSN 来自 config.yaml / AGENT_ 环境变量）
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// ② 升级：file source 顺序执行 migrations/NNN_xxx.up.sql，
	//    已应用的记录在 schema_migrations 表（golang-migrate 自动维护，幂等）
	m, err := migrate.New("file://migrations", cfg.PG.DSN)
	if err != nil {
		return fmt.Errorf("new migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if down > 0 {
		if err := m.Steps(-down); err != nil {
			return fmt.Errorf("step down %d: %w", down, err)
		}
		fmt.Printf("rolled back %d version(s)\n", down)
		return nil
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("up: %w", err)
	}
	fmt.Println("migrations applied")

	// ③ 种子管理员：users 表为空才插入（幂等，密码哈希在代码层生成，
	//    与 002_seed.up.sql 头部注释的设计一致）
	if err := seedAdmin(cfg.PG.DSN); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	return nil
}

// seedAdmin 插入默认管理员 admin / admin123（仅当 users 表为空）。
func seedAdmin(dsn string) error {
	db, err := infra.NewPG(disabledCtx(), &config.PG{DSN: dsn, MaxOpenConns: 2, MaxIdleConns: 1})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 唯一条记录插入：存在任何用户则跳过（与种子 SQL 的幂等约定一致）
	res := db.WithContext(disabledCtx()).Exec(`
		INSERT INTO users (username, email, password_hash, role, status)
		SELECT 'admin', 'admin@agent.local', ?, 'admin', 1
		WHERE NOT EXISTS (SELECT 1 FROM users)`, string(hash))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		fmt.Println("seeded admin user: admin / admin123 (请在管理页修改密码)")
	} else {
		fmt.Println("users table not empty, skip seed")
	}
	return nil
}

// disabledCtx 迁移场景不需要可取消上下文，给一个空实现保持 infra.NewPG 签名。
func disabledCtx() (ctx context.Context) {
	return context.Background()
}
