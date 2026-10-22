package clients

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"dashboard/global/config"
)

// fakeSnippet 은 snippet 인증 흐름만 흉내 낸다: 로그인마다 새 refresh 토큰(=세션 행)을 발급하고,
// refresh 는 토큰을 바꾸지 않고 access 토큰만 새로 준다. expire() 로 access 토큰을 전부 만료시킨다.
type fakeSnippet struct {
	mu          sync.Mutex
	logins      int
	refreshes   int
	refreshMode string // "ok" | "reject" | "down"
	seq         int
	valid       map[string]bool // 유효한 access 토큰
	sessions    map[string]bool // 유효한 refresh 토큰
}

func newFakeSnippet(t *testing.T) *fakeSnippet {
	f := &fakeSnippet{refreshMode: "ok", valid: map[string]bool{}, sessions: map[string]bool{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)

	oldURL, oldEmail, oldPassword := config.SnippetApiUrl, config.SnippetEmail, config.SnippetPassword
	config.SnippetApiUrl, config.SnippetEmail, config.SnippetPassword = srv.URL, "me@example.com", "pw"
	snippetToken, snippetRefreshToken = "", ""
	t.Cleanup(func() {
		config.SnippetApiUrl, config.SnippetEmail, config.SnippetPassword = oldURL, oldEmail, oldPassword
		snippetToken, snippetRefreshToken = "", ""
	})
	return f
}

func (f *fakeSnippet) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.Method == "POST" && r.URL.Path == "/auth/login":
		f.logins++
		f.seq++
		access, refresh := fmt.Sprintf("access-%d", f.seq), fmt.Sprintf("refresh-%d", f.seq)
		f.valid[access], f.sessions[refresh] = true, true
		json.NewEncoder(w).Encode(map[string]string{"token": access, "refreshToken": refresh})

	case r.Method == "POST" && r.URL.Path == "/auth/refresh":
		f.refreshes++
		var req struct {
			RefreshToken string `json:"refreshToken"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if f.refreshMode == "down" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if f.refreshMode == "reject" || !f.sessions[req.RefreshToken] {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"message":"유효하지 않은 리프레시 토큰입니다."}`))
			return
		}
		f.seq++
		access := fmt.Sprintf("access-%d", f.seq)
		f.valid[access] = true
		json.NewEncoder(w).Encode(map[string]string{"token": access, "refreshToken": req.RefreshToken})

	case r.Method == "GET" && r.URL.Path == "/data":
		if !f.valid[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")] {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`"ok"`))

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeSnippet) expire() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.valid = map[string]bool{}
}

func (f *fakeSnippet) counts() (logins, refreshes int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.logins, f.refreshes
}

func (f *fakeSnippet) setRefreshMode(mode string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshMode = mode
}

func mustSnippetGet(t *testing.T) {
	t.Helper()
	buf, err := snippetGet("/data")
	if err != nil || string(buf) != `"ok"` {
		t.Fatalf("snippetGet: buf=%q err=%v", buf, err)
	}
}

func TestSnippetGetLogsInOnceAndReusesToken(t *testing.T) {
	f := newFakeSnippet(t)
	mustSnippetGet(t)
	mustSnippetGet(t)
	if logins, refreshes := f.counts(); logins != 1 || refreshes != 0 {
		t.Errorf("logins=%d refreshes=%d, want 1/0", logins, refreshes)
	}
}

func TestSnippetGetRefreshesExpiredTokenWithoutLogin(t *testing.T) {
	f := newFakeSnippet(t)
	mustSnippetGet(t)
	for hour := 0; hour < 3; hour++ { // 1시간마다 access 토큰 만료 → refresh 만으로 버틴다
		f.expire()
		mustSnippetGet(t)
	}
	if logins, refreshes := f.counts(); logins != 1 || refreshes != 3 {
		t.Errorf("logins=%d refreshes=%d, want 1/3", logins, refreshes)
	}
}

func TestSnippetGetLogsInAgainWhenRefreshRejected(t *testing.T) {
	f := newFakeSnippet(t)
	mustSnippetGet(t)
	f.setRefreshMode("reject") // 세션 행이 만료·삭제된 상황
	f.expire()
	mustSnippetGet(t)
	if logins, refreshes := f.counts(); logins != 2 || refreshes != 1 {
		t.Errorf("logins=%d refreshes=%d, want 2/1", logins, refreshes)
	}

	// 새로 로그인한 세션으로 다시 refresh 가 이어진다
	f.setRefreshMode("ok")
	f.expire()
	mustSnippetGet(t)
	if logins, refreshes := f.counts(); logins != 2 || refreshes != 2 {
		t.Errorf("after recovery: logins=%d refreshes=%d, want 2/2", logins, refreshes)
	}
}

func TestSnippetGetDoesNotLoginWhenRefreshServerDown(t *testing.T) {
	f := newFakeSnippet(t)
	mustSnippetGet(t)
	f.setRefreshMode("down") // 배포 재시작 중 502
	f.expire()
	if _, err := snippetGet("/data"); err == nil {
		t.Fatal("want error while snippet is down")
	}
	if logins, _ := f.counts(); logins != 1 {
		t.Errorf("logins=%d, want 1 (no login on 5xx)", logins)
	}

	// 복구되면 기존 refresh 토큰을 그대로 쓴다
	f.setRefreshMode("ok")
	mustSnippetGet(t)
	if logins, refreshes := f.counts(); logins != 1 || refreshes != 2 {
		t.Errorf("after recovery: logins=%d refreshes=%d, want 1/2", logins, refreshes)
	}
}

func TestSnippetGetConcurrent401sRefreshOnce(t *testing.T) {
	f := newFakeSnippet(t)
	mustSnippetGet(t)
	f.expire()

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := snippetGet("/data"); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if logins, refreshes := f.counts(); logins != 1 || refreshes != 1 {
		t.Errorf("logins=%d refreshes=%d, want 1/1", logins, refreshes)
	}
}
