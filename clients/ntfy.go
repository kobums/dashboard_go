package clients

// ntfy 발송 — iPhone ntfy 앱(무료)으로 APNs 푸시를 보낸다.
// JSON publish 방식(POST {server} 루트) — 헤더 방식과 달리 한글 제목/본문 인코딩 문제가 없다.
// NTFY_TOPIC 이 비어 있으면 조용히 건너뛴다 (opt-in).
// 공개 ntfy.sh 는 토픽 이름이 곧 비밀번호 — 추측 불가능한 긴 토픽을 쓸 것.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"dashboard/global/config"
)

// SendNtfy 는 제목/본문/클릭 URL/우선순위(1~5, 0=기본)로 알림을 발행한다.
func SendNtfy(title, message, click string, priority int) error {
	if config.NtfyTopic == "" {
		return nil
	}

	payload := map[string]interface{}{
		"topic":   config.NtfyTopic,
		"title":   title,
		"message": message,
	}
	if click != "" {
		payload["click"] = click
	}
	if priority > 0 {
		payload["priority"] = priority
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(config.NtfyServer, "application/json", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy publish failed: %v", resp.Status)
	}
	return nil
}
