package rest

// 신규 가입 알림용 조회 — services/signupnotify.go 가 5분마다 호출한다.
// 서비스별 워터마크(마지막으로 본 가입 시각, KST) 이후 가입 수를 센다.
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"time"

	"dashboard/global/log"
	"dashboard/models"
)

type NewSignup struct {
	Key   string
	Name  string
	Count int // 워터마크 이후 가입
	Today int // 오늘 누적 가입
}

// NewSignupsSince 는 since(서비스 키 → 마지막으로 본 가입 시각) 이후 가입을 센다.
// 반환하는 next 는 갱신된 워터마크. since 에 없는 서비스는 기준선만 잡고 알리지 않는다(첫 실행·서비스 추가).
// 조회에 실패한 서비스는 워터마크를 그대로 둬 다음 회차에 다시 센다.
func NewSignupsSince(since map[string]string) ([]NewSignup, map[string]string) {
	conn := models.NewConnection()
	defer conn.Close()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	next := map[string]string{}
	var found []NewSignup

	for _, s := range signupServices {
		prev, seen := since[s.Key]
		col := s.Column
		var count, todayCount int
		var latest *string
		// 미래 시각 행(잘못 들어간 데이터)이 워터마크를 막지 않게 now 이하만 본다
		err := conn.Conn.QueryRow(
			"SELECT COALESCE(SUM("+col+" > ?), 0), COALESCE(SUM("+col+" >= ?), 0), "+
				// 워터마크는 마이크로초까지 — 초 단위로 자르면 소수 초를 저장하는 컬럼(risksync·Prisma)의
				// 마지막 행이 계속 "워터마크보다 큼"이 되어 5분마다 재발송된다
				"DATE_FORMAT(MAX("+col+"), '%Y-%m-%d %H:%i:%s.%f') FROM "+s.Table+" WHERE "+col+" <= ?",
			prev, today.Format(dtFmt), now.Format(dtFmt),
		).Scan(&count, &todayCount, &latest)
		if err != nil {
			log.Error().Str("service", s.Key).Str("part", "signup-notify").Msg(err.Error())
			if seen {
				next[s.Key] = prev
			}
			continue
		}

		switch {
		case latest != nil:
			next[s.Key] = *latest
		case seen:
			next[s.Key] = prev
		default:
			next[s.Key] = "" // 가입 0명 — 기준선은 "처음부터"
		}
		if seen && count > 0 {
			found = append(found, NewSignup{Key: s.Key, Name: s.Name, Count: count, Today: todayCount})
		}
	}
	return found, next
}
