// Command seed 为开发/演示环境生成可复现的健康演示数据。
// 默认 dry-run；必须显式传入 -yes 才会写入数据库。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"ai_health/internal/seed"
)

func main() {
	var (
		yes        bool
		anchor     string
		randomSeed int64
	)
	flag.BoolVar(&yes, "yes", false, "实际写入数据库；不传时仅 dry-run")
	flag.StringVar(&anchor, "anchor", seed.DefaultAnchor, "演示日期锚点（YYYY-MM-DD，仅 UTC）")
	flag.Int64Var(&randomSeed, "random-seed", seed.DefaultRandomSeed, "固定随机种子，用于复现数据")
	flag.Parse()

	dsn := os.Getenv("AIH_SEED_MYSQL_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "必须通过 AIH_SEED_MYSQL_DSN 显式提供开发/演示数据库 DSN")
		os.Exit(2)
	}

	err := seed.Run(context.Background(), seed.Options{
		DSN:        dsn,
		Yes:        yes,
		Anchor:     anchor,
		RandomSeed: randomSeed,
		Out:        os.Stdout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed 失败:", err)
		os.Exit(1)
	}
}
