package clients

// 독서 진행 스냅샷 — 독서 세션(타이머) 없이 진행률만 올리거나 완독 처리한 날도 「읽은 날」로 인정하기 위함.
// snippet 은 책별 현재 쪽수(readPage)만 주고 변경 이력이 없어서, 날짜별 readPage 를 fetchcache_tb 에 쌓아두고
// 기준일 마지막 상태와 그 이전 마지막 상태를 비교해 「그날 진행한 쪽수」를 구한다.
// 완독은 snippet 이 상태 변경 시 endDate 를 찍어주므로 스냅샷 없이 판정한다.
// 캐시가 아니라 영구 저장 용도라 GetCached 대신 매니저를 직접 쓴다 (push_subscriptions 와 같은 방식).

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"dashboard/global/log"
	"dashboard/models"
)

const (
	readingSnapKey      = "reading_progress_snapshots"
	readingSnapKeepDays = 60
)

// readingSnapshots: 날짜(YYYY-MM-DD) → userbook id → 그날 마지막으로 본 readPage
type readingSnapshots map[string]map[string]int

type snippetUserBook struct {
	Id       int64  `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	ReadPage int    `json:"readPage"`
	EndDate  string `json:"endDate"`
}

var readingSnapMutex sync.Mutex

// 마지막 스냅샷 메모리 보관 — 홈 배너가 /notify/check 를 부를 때마다 snippet·원격 DB 왕복을 하지 않도록.
// 맵은 매번 새로 만들고 저장 후엔 수정하지 않으므로 읽기 공유가 안전하다.
const readingSnapFresh = 5 * time.Minute

var lastReadingSnap struct {
	at    time.Time
	books []snippetUserBook
	snaps readingSnapshots
}

func loadReadingSnapshots(conn *models.Connection) readingSnapshots {
	snaps := readingSnapshots{}
	manager := models.NewFetchcacheManager(conn)
	item := manager.GetWhere([]interface{}{models.Where{Column: "cachekey", Value: readingSnapKey, Compare: "="}})
	if item == nil || item.Payload == "" {
		return snaps
	}
	if err := json.Unmarshal([]byte(item.Payload), &snaps); err != nil {
		log.Error().Str("reading", readingSnapKey).Msg(err.Error())
		return readingSnapshots{}
	}
	return snaps
}

func saveReadingSnapshots(conn *models.Connection, snaps readingSnapshots) {
	buf, err := json.Marshal(snaps)
	if err != nil {
		log.Error().Str("reading", readingSnapKey).Msg(err.Error())
		return
	}
	manager := models.NewFetchcacheManager(conn)
	now := time.Now().Format(cacheTimeFormat)
	item := manager.GetWhere([]interface{}{models.Where{Column: "cachekey", Value: readingSnapKey, Compare: "="}})
	if item == nil {
		manager.Insert(&models.Fetchcache{Cachekey: readingSnapKey, Payload: string(buf), Fetchedat: now})
	} else {
		item.Payload = string(buf)
		item.Fetchedat = now
		manager.Update(item)
	}
}

// snapshotReadingProgress 는 snippet 책 목록을 받아 오늘 스냅샷을 갱신하고 (책 목록, 전체 스냅샷)을 돌려준다.
func snapshotReadingProgress() ([]snippetUserBook, readingSnapshots, error) {
	buf, err := snippetGet("/userbooks")
	if err != nil {
		return nil, nil, err
	}
	var books []snippetUserBook
	if err := json.Unmarshal(buf, &books); err != nil {
		return nil, nil, err
	}

	readingSnapMutex.Lock()
	defer readingSnapMutex.Unlock()

	conn := models.NewConnection()
	defer conn.Close()

	snaps := loadReadingSnapshots(conn)
	today := map[string]int{}
	for _, b := range books {
		today[strconv.FormatInt(b.Id, 10)] = b.ReadPage
	}
	snaps[time.Now().Format("2006-01-02")] = today

	cutoff := time.Now().AddDate(0, 0, -readingSnapKeepDays).Format("2006-01-02")
	for d := range snaps {
		if d < cutoff {
			delete(snaps, d)
		}
	}
	saveReadingSnapshots(conn, snaps)

	lastReadingSnap.at = time.Now()
	lastReadingSnap.books = books
	lastReadingSnap.snaps = snaps
	return books, snaps, nil
}

// recentReadingSnapshot 은 readingSnapFresh 안의 스냅샷이 있으면 재사용하고, 없으면 새로 찍는다.
func recentReadingSnapshot() ([]snippetUserBook, readingSnapshots, error) {
	readingSnapMutex.Lock()
	at, books, snaps := lastReadingSnap.at, lastReadingSnap.books, lastReadingSnap.snaps
	readingSnapMutex.Unlock()
	if snaps != nil && time.Since(at) < readingSnapFresh && at.Format("2006-01-02") == time.Now().Format("2006-01-02") {
		return books, snaps, nil
	}
	return snapshotReadingProgress()
}

// SnapshotReadingProgress 는 스케줄러용 — 오늘 스냅샷만 갱신한다.
func SnapshotReadingProgress() error {
	_, _, err := snapshotReadingProgress()
	return err
}

// ReadingActivity 는 기준일(date)에 독서 세션 외 활동을 계산한다.
//
//	pages    : 직전 스냅샷 대비 readPage 증가분 합 (직전 스냅샷이 없으면 0 — 비교 불가)
//	finished : 그날 완독 처리한 책 제목
func ReadingActivity(date string) (pages int, finished []string, err error) {
	books, snaps, err := recentReadingSnapshot()
	if err != nil {
		return 0, nil, err
	}
	return progressPages(snaps, date), finishedOn(books, date), nil
}

// finishedOn 은 date 에 완독 처리한 책 제목 (snippet 은 상태 변경 시 endDate 를 찍는다).
func finishedOn(books []snippetUserBook, date string) []string {
	finished := []string{}
	for _, b := range books {
		if (b.Status == "completed" || b.Status == "done") && len(b.EndDate) >= 10 && b.EndDate[:10] == date {
			finished = append(finished, b.Title)
		}
	}
	return finished
}

// progressPages 는 date 스냅샷과 그 이전 가장 최근 스냅샷의 readPage 증가분 합.
// 어느 한쪽이 없으면 비교 불가 → 0.
func progressPages(snaps readingSnapshots, date string) int {
	after, ok := snaps[date]
	if !ok {
		return 0
	}
	prevDate := ""
	for d := range snaps {
		if d < date && d > prevDate {
			prevDate = d
		}
	}
	if prevDate == "" {
		return 0
	}
	before := snaps[prevDate]

	pages := 0
	for id, page := range after {
		prev, ok := before[id]
		if !ok {
			continue // 기준일에 새로 등록한 책 — 이미 읽은 쪽수로 등록했을 수 있어 제외
		}
		if page > prev {
			pages += page - prev
		}
	}
	return pages
}
