package main

// 개발 컨트리뷰션 과거 전체 백필 (일회성).
// dashboard_go 디렉토리에서: go run ./cmd/backfill
// (.env.yml 의 githubToken/gitlabToken 사용, DB 는 config 의 database 설정)

import (
	"fmt"
	"os"

	"dashboard/clients"
	_ "dashboard/global/log" // init 에서 config 로드
)

func main() {
	result, err := clients.BackfillDev()
	if err != nil {
		fmt.Fprintln(os.Stderr, "backfill failed:", err)
		os.Exit(1)
	}
	fmt.Printf("백필 완료: github %d일, gitlab %d일 (활동일 기준)\n", result["github"], result["gitlab"])
}
