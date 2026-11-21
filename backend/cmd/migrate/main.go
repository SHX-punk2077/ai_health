package main

import (
	"flag"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// 迁移入口，便于在容器里用同一份二进制执行 up/down/version。
// migrate -path ./migrations -database "mysql://user:pass@tcp(host:3306)/ai_health?multiStatements=true" up
func main() {
	path := flag.String("path", "migrations", "迁移脚本目录")
	dsn := flag.String("database", "", "mysql://user:pass@tcp(host:3306)/ai_health?multiStatements=true")
	flag.Parse()

	if *dsn == "" {
		log.Fatal("-database 不能为空")
	}

	m, err := migrate.New("file://"+*path, *dsn)
	if err != nil {
		log.Fatalf("init migrate: %v", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migrate up: %v", err)
	}

	v, dirty, err := m.Version()
	if err != nil {
		log.Printf("migrate done, version unknown: %v", err)
		return
	}
	log.Printf("migrate done, version=%d dirty=%v", v, dirty)
}
