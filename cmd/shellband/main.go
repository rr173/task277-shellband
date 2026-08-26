// Command shellband 启动贝壳同位素季节生长带对齐服务 HTTP 服务，
// 或在 --smoke-test 模式下运行端到端自检（真实建库、关闭重开验证持久化）。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"task277-shellband/internal/httpapi"
	"task277-shellband/internal/service"
	"task277-shellband/internal/store"
)

func main() {
	var (
		addr       string
		dbPath     string
		smokeTest  bool
	)
	flag.StringVar(&addr, "addr", ":8080", "HTTP 监听地址")
	flag.StringVar(&dbPath, "db", "./shellband.db", "SQLite 数据库文件路径（空字符串使用内存库）")
	flag.BoolVar(&smokeTest, "smoke-test", false, "运行端到端自检后退出（0=成功）")
	flag.Parse()

	if smokeTest {
		if err := runSmokeTest(); err != nil {
			fmt.Fprintln(os.Stderr, "SMOKE-TEST FAILED:", err)
			os.Exit(1)
		}
		fmt.Println("SMOKE-TEST PASSED")
		os.Exit(0)
	}

	db := dbPath
	if db == "" {
		db = ":memory:"
	}
	st, err := store.Open(db)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	svc := service.New(st)
	srv := httpapi.NewServer(svc)
	log.Printf("shellband listening on %s (db=%s)", addr, db)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("http server: %v", err)
	}
}
