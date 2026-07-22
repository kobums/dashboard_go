package clients

// fetchcache_tb 기반 stale-while-revalidate 캐시.
// - TTL 안이면 캐시 그대로 반환
// - TTL 지났지만 캐시가 있으면 캐시를 즉시 반환하고 백그라운드에서 갱신 (single-flight)
// - 캐시가 아예 없으면 동기로 fetch

import (
	"sync"
	"time"

	"dashboard/global/log"
	"dashboard/models"
)

const cacheTimeFormat = "2006-01-02 15:04:05"

var refreshing sync.Map // key → true (갱신 goroutine 중복 방지)

func GetCached(key string, ttl time.Duration, fetch func() ([]byte, error)) ([]byte, error) {
	conn := models.NewConnection()
	defer conn.Close()

	manager := models.NewFetchcacheManager(conn)
	item := manager.GetWhere([]interface{}{models.Where{Column: "cachekey", Value: key, Compare: "="}})

	if item != nil {
		fetchedAt, err := time.ParseInLocation(cacheTimeFormat, item.Fetchedat, time.Local)
		if err == nil && time.Since(fetchedAt) < ttl {
			return []byte(item.Payload), nil
		}

		// stale — 즉시 반환하고 백그라운드 갱신
		go refreshCache(key, fetch)
		return []byte(item.Payload), nil
	}

	// 캐시 없음 — 동기 fetch
	buf, err := fetch()
	if err != nil {
		return nil, err
	}
	saveCache(key, buf)
	return buf, nil
}

func refreshCache(key string, fetch func() ([]byte, error)) {
	if _, loaded := refreshing.LoadOrStore(key, true); loaded {
		return
	}
	defer refreshing.Delete(key)

	buf, err := fetch()
	if err != nil {
		log.Error().Str("cache", key).Msg(err.Error())
		return
	}
	saveCache(key, buf)
}

func saveCache(key string, payload []byte) {
	conn := models.NewConnection()
	defer conn.Close()

	manager := models.NewFetchcacheManager(conn)
	now := time.Now().Format(cacheTimeFormat)

	item := manager.GetWhere([]interface{}{models.Where{Column: "cachekey", Value: key, Compare: "="}})
	if item == nil {
		manager.Insert(&models.Fetchcache{Cachekey: key, Payload: string(payload), Fetchedat: now})
	} else {
		item.Payload = string(payload)
		item.Fetchedat = now
		manager.Update(item)
	}
}
