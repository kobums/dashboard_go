package services

// 노션 「식단 기록」 동기화 스케줄러 — 기동 직후 1회 + 이후 10분마다.
// 웨이트(3시간)보다 자주 돈다: 노션에서 끼니를 적고 바로 대시보드를 여는 일이 잦고,
// 바뀌지 않은 페이지는 본문 캐시(dietDayCache)를 써서 노션 호출이 query 몇 번뿐이다.

import (
	"fmt"
	"time"

	"dashboard/clients"
	"dashboard/global/config"
	"dashboard/global/log"
)

const dietSyncInterval = 10 * time.Minute

func StartDietSync() {
	if config.NotionToken == "" {
		log.Info().Str("service", "diet-sync").Msg("disabled (NOTION_TOKEN 미설정)")
		return
	}
	log.Info().Str("service", "diet-sync").Msg("Start Service (every 10m)")
	go func() {
		for {
			runDietSync()
			time.Sleep(dietSyncInterval)
		}
	}()
}

func runDietSync() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Str("service", "diet-sync").Msg(fmt.Sprintf("panic: %v", r))
		}
	}()
	if err := clients.SyncDiet(); err != nil {
		log.Error().Str("service", "diet-sync").Msg(err.Error())
	}
}
