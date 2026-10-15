package services

// 독서 진행 스냅샷 — 기동 직후 1회 + 이후 20분마다 snippet 책별 쪽수를 저장한다.
// 하루 마지막 스냅샷이 그날의 최종 진행 상태가 되므로 간격이 짧을수록 자정 경계 오차가 줄어든다.
// 알림 판정(clients.ReadingActivity)이 이 이력으로 「세션 없이 진행률만 올린 날」을 찾는다.

import (
	"fmt"
	"time"

	"dashboard/clients"
	"dashboard/global/config"
	"dashboard/global/log"
)

const readingSnapInterval = 20 * time.Minute

func StartReadingSnapshot() {
	if config.SnippetEmail == "" {
		log.Info().Str("service", "reading-snapshot").Msg("disabled (snippet 계정 미설정)")
		return
	}
	log.Info().Str("service", "reading-snapshot").Msg("Start Service (every 20m)")
	go func() {
		for {
			runReadingSnapshot()
			time.Sleep(readingSnapInterval)
		}
	}()
}

func runReadingSnapshot() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Str("service", "reading-snapshot").Msg(fmt.Sprintf("panic: %v", r))
		}
	}()
	if err := clients.SnapshotReadingProgress(); err != nil {
		log.Error().Str("service", "reading-snapshot").Msg(err.Error())
	}
}
