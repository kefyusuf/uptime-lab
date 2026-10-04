package monitoringhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

type inventoryStub struct {
	calls int
	input application.InventoryInput
	page  application.InventoryCandidates
	err   error
}

func (s *inventoryStub) Execute(_ context.Context, input application.InventoryInput) (application.InventoryCandidates, error) {
	s.calls++
	s.input = input
	return s.page, s.err
}
func httpInventoryCandidate(t *testing.T, index int, target string) ports.InventoryCandidate {
	t.Helper()
	id, err := domain.ParseMonitorID(fmt.Sprintf("018f22d3-1d6a-7cc0-a37b-%012x", 0x1000-index))
	if err != nil {
		t.Fatal(err)
	}
	return ports.InventoryCandidate{ID: id, CreatedAt: time.Date(2026, 10, 5, 0, 0, 0, 123456000, time.UTC), TargetURL: target, TargetBytes: int64(len(target))}
}
func TestInventoryQueryRawGrammar(t *testing.T) {
	item := httpInventoryCandidate(t, 0, "http://web/")
	cursor, _ := application.EncodeInventoryCursor(ports.InventoryAnchor{ID: item.ID, CreatedAt: item.CreatedAt})
	for _, raw := range []string{"", "limit=1", "limit=20", "limit=50", "cursor=" + cursor, "limit=20&cursor=" + cursor, "cursor=" + cursor + "&limit=20"} {
		if _, err := parseInventoryQuery(raw); err != nil {
			t.Fatal(raw, err)
		}
	}
	for _, raw := range []string{"limit=0", "limit=51", "limit=020", "limit=+20", "limit=-1", "limit=20&limit=20", "cursor=", "cursor=" + cursor + "&cursor=" + cursor, "x=1", "limit=%32%30", "limit=20;cursor=" + cursor, "limit=20&", "limit=20 ", "limit=20+", "limit=20&x=é", strings.Repeat("a", 128), strings.Repeat("a", 129)} {
		if _, err := parseInventoryQuery(raw); err != application.ErrInvalidInventoryQuery {
			t.Fatal(raw, err)
		}
	}
}
func TestInventoryPageByteBudgetAndComma(t *testing.T) {
	first := httpInventoryCandidate(t, 0, "http://web/")
	encoded, _ := json.Marshal(monitorResponse{ID: first.ID.String(), TargetURL: first.TargetURL, CreatedAt: first.CreatedAt})
	padding := 245760 - 128 - len(encoded)
	first.TargetURL += strings.Repeat("x", padding)
	first.TargetBytes = int64(len(first.TargetURL))
	body, err := encodeInventoryPage([]ports.InventoryCandidate{first}, 20)
	if err != nil || len(body) > 245760 || !strings.Contains(string(body), first.TargetURL) {
		t.Fatal(len(body), err)
	}
	first.TargetURL += "x"
	first.TargetBytes++
	if _, err := encodeInventoryPage([]ports.InventoryCandidate{first}, 20); err != errInventoryItemTooLarge {
		t.Fatal(err)
	}
	second := httpInventoryCandidate(t, 1, "http://web/?value=é&<>")
	normal := httpInventoryCandidate(t, 0, "http://web/")
	secondJSON, _ := json.Marshal(monitorResponse{ID: second.ID.String(), TargetURL: second.TargetURL, CreatedAt: second.CreatedAt})
	firstJSON, _ := json.Marshal(monitorResponse{ID: normal.ID.String(), TargetURL: normal.TargetURL, CreatedAt: normal.CreatedAt})
	normal.TargetURL += strings.Repeat("x", 245760-128-len(firstJSON)-len(secondJSON)-1)
	normal.TargetBytes = int64(len(normal.TargetURL))
	body, err = encodeInventoryPage([]ports.InventoryCandidate{normal, second}, 20)
	var page struct {
		Items      []monitorResponse `json:"items"`
		NextCursor *string           `json:"nextCursor"`
	}
	if err != nil || json.Unmarshal(body, &page) != nil || len(page.Items) != 2 || page.NextCursor != nil || page.Items[1].TargetURL != second.TargetURL {
		t.Fatal(len(body), page, err)
	}
	normal.TargetURL += "x"
	normal.TargetBytes++
	body, err = encodeInventoryPage([]ports.InventoryCandidate{normal, second}, 20)
	if err != nil || json.Unmarshal(body, &page) != nil || len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatal(page, err)
	}
	anchor, err := application.DecodeInventoryCursor(*page.NextCursor)
	if err != nil || anchor.ID != normal.ID {
		t.Fatal(anchor, err)
	}
}
func TestInventoryOversizedLookaheadDoesNotFailPrefix(t *testing.T) {
	normal := httpInventoryCandidate(t, 0, "http://web/")
	huge := httpInventoryCandidate(t, 1, "")
	huge.Oversized = true
	huge.TargetBytes = 245761
	for _, limit := range []int{1, 20} {
		body, err := encodeInventoryPage([]ports.InventoryCandidate{normal, huge}, limit)
		if err != nil || !strings.Contains(string(body), normal.ID.String()) || strings.Contains(string(body), huge.ID.String()) || strings.Contains(string(body), `"nextCursor":null`) {
			t.Fatal(string(body), err)
		}
	}
	body, err := encodeInventoryPage(nil, 20)
	if err != nil || string(body) != `{"items":[],"nextCursor":null}` {
		t.Fatal(string(body), err)
	}
	items := make([]ports.InventoryCandidate, 51)
	for i := range items {
		items[i] = httpInventoryCandidate(t, i, "http://web/")
	}
	body, err = encodeInventoryPage(items, 50)
	var page struct {
		Items      []monitorResponse
		NextCursor *string
	}
	if err != nil || json.Unmarshal(body, &page) != nil || len(page.Items) != 50 || page.NextCursor == nil {
		t.Fatal(page, err)
	}
}
func TestInventoryFirstOversizedReturns500(t *testing.T) {
	for _, item := range []ports.InventoryCandidate{httpInventoryCandidate(t, 0, "http://web/?"+strings.Repeat("&", 50000)), {ID: httpInventoryCandidate(t, 0, "").ID, CreatedAt: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), Oversized: true, TargetBytes: 245761}} {
		stub := &inventoryStub{page: application.InventoryCandidates{Items: []ports.InventoryCandidate{item}, Limit: 20}}
		handler := NewHandlerWithInventory(nil, nil, nil, nil, stub)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/monitors", nil))
		if rec.Code != 500 || rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), "A registered monitor exceeds the inventory response limit.") || strings.Contains(rec.Body.String(), item.ID.String()) {
			t.Fatal(rec.Code, rec.Body.String())
		}
	}
}
func TestInventoryCollectionMethodCacheAndBody(t *testing.T) {
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{{"GET", "/monitors", "", 200}, {"GET", "/monitors?limit=20", "", 200}, {"GET", "/monitors?", "", 400}, {"GET", "/monitors?limit=020", "", 400}, {"GET", "/monitors", "x", 400}, {"GET", "/%6donitors", "", 400}, {"HEAD", "/monitors", "", 405}, {"OPTIONS", "/monitors", "", 405}} {
		stub := &inventoryStub{page: application.InventoryCandidates{Limit: 20}}
		handler := NewHandlerWithInventory(nil, nil, nil, nil, stub)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.status || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(tc, rec.Code, rec.Header())
		}
		if tc.status != 200 && stub.calls != 0 {
			t.Fatal(tc, stub.calls)
		}
		if tc.status == 405 && (rec.Header().Get("Allow") != "GET, POST" || rec.Body.Len() != 0) {
			t.Fatal(rec.Header(), rec.Body.String())
		}
	}
	stub := &inventoryStub{page: application.InventoryCandidates{Limit: 20}}
	handler := NewHandlerWithInventory(nil, nil, nil, nil, stub)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/monitors", nil)
	req.TransferEncoding = []string{"chunked"}
	handler.ServeHTTP(rec, req)
	if rec.Code != 400 || stub.calls != 0 {
		t.Fatal(rec.Code, stub.calls)
	}
}

func TestInventoryErrorsAreSanitizedAndPOSTQueryIsPreserved(t *testing.T) {
	for _, cause := range []error{application.ErrPersistence, application.ErrInvalidInventoryQuery} {
		stub := &inventoryStub{err: cause}
		rec := httptest.NewRecorder()
		NewHandlerWithInventory(nil, nil, nil, nil, stub).ServeHTTP(rec, httptest.NewRequest("GET", "/monitors", nil))
		want := 500
		if cause == application.ErrInvalidInventoryQuery {
			want = 400
		}
		if rec.Code != want || rec.Header().Get("Cache-Control") != "no-store" || strings.Contains(rec.Body.String(), cause.Error()) {
			t.Fatal(rec.Code, rec.Body.String())
		}
	}
	item := httpInventoryCandidate(t, 0, "http://web/")
	target, err := domain.NewTargetURL(item.TargetURL)
	if err != nil {
		t.Fatal(err)
	}
	monitor, err := domain.NewMonitor(item.ID, target, item.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	register := &registerMonitorStub{monitor: monitor}
	req := httptest.NewRequest("POST", "/monitors?legacy=allowed", strings.NewReader(`{"targetUrl":"http://web/"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	NewHandlerWithInventory(register, nil, nil, nil, &inventoryStub{}).ServeHTTP(rec, req)
	if rec.Code != 201 || register.calls != 1 {
		t.Fatal(rec.Code, register.calls)
	}
}

func TestInventoryFiftyRowCommaBoundary(t *testing.T) {
	items := make([]ports.InventoryCandidate, 50)
	used := 128 + 49
	for i := range items {
		items[i] = httpInventoryCandidate(t, i, "http://web/")
		raw, err := json.Marshal(monitorResponse{ID: items[i].ID.String(), TargetURL: items[i].TargetURL, CreatedAt: items[i].CreatedAt})
		if err != nil {
			t.Fatal(err)
		}
		used += len(raw)
	}
	items[0].TargetURL += strings.Repeat("x", 245760-used)
	items[0].TargetBytes = int64(len(items[0].TargetURL))
	var page struct {
		Items      []monitorResponse
		NextCursor *string
	}
	body, err := encodeInventoryPage(items, 50)
	if err != nil || json.Unmarshal(body, &page) != nil || len(page.Items) != 50 || page.NextCursor != nil || len(body) > 245760 {
		t.Fatal(len(page.Items), err)
	}
	items[0].TargetURL += "x"
	items[0].TargetBytes++
	body, err = encodeInventoryPage(items, 50)
	if err != nil || json.Unmarshal(body, &page) != nil || len(page.Items) != 49 || page.NextCursor == nil {
		t.Fatal(len(page.Items), err)
	}
	key, err := application.DecodeInventoryCursor(*page.NextCursor)
	if err != nil || key.ID != items[48].ID {
		t.Fatal(key, err)
	}
}
