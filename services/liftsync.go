package services

// 노션 운동 기록 동기화 스케줄러 — 기동 직후 1회 + 이후 3시간마다 전체 재동기화.
// 헬스장에서 기록한 직후 바로 보고 싶으면 웨이트 화면의 「지금 동기화」(POST /api/lift/sync).

import (
	"fmt"
	"time"

	"dashboard/clients"
	"dashboard/global/config"
	"dashboard/global/log"
)

const liftSyncInterval = 3 * time.Hour

func StartLiftSync() {
	if config.NotionToken == "" {
		log.Info().Str("service", "lift-sync").Msg("disabled (NOTION_TOKEN 미설정)")
		return
	}
	log.Info().Str("service", "lift-sync").Msg("Start Service (every 3h)")
	go func() {
		for {
			runLiftSync()
			time.Sleep(liftSyncInterval)
		}
	}()
}

func runLiftSync() {
	// DB/노션 오류로 죽어도 루프는 살아야 한다.
	defer func() {
		if r := recover(); r != nil {
			log.Error().Str("service", "lift-sync").Msg(fmt.Sprintf("panic: %v", r))
		}
	}()
	clients.SyncLift() // 오류는 SyncLift 안에서 로그 + 상태에 남긴다
}
