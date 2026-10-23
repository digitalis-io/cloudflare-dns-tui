package cloudflare_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/digitalis-io/cloudflare-dns-tui/internal/cloudflare"
)

func apiResponse(t *testing.T, result any, totalPages int) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"success": true,
			"errors":  []any{},
			"result":  result,
			"result_info": map[string]any{
				"total_pages": totalPages,
			},
		}
		if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
			t.Errorf("encode response: %v", encErr)
		}
	}
}

func TestZones_Sorted(t *testing.T) {
	srv := httptest.NewServer(apiResponse(t, []map[string]any{
		{"id": "z2", "name": "beta.example.com"},
		{"id": "z1", "name": "alpha.example.com"},
	}, 1))
	defer srv.Close()

	client := cloudflare.NewClientWithBase("tok", srv.URL, srv.Client())
	zones, err := client.Zones(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(zones) != 2 {
		t.Fatalf("want 2 zones, got %d", len(zones))
	}
	if zones[0].Name != "alpha.example.com" {
		t.Errorf("want alpha first, got %s", zones[0].Name)
	}
}

func TestZones_EmptyToken(t *testing.T) {
	client := cloudflare.NewClientWithBase("", "http://localhost", http.DefaultClient)
	_, err := client.Zones(context.Background())
	if err == nil {
		t.Fatal("want error for empty token")
	}
}

func TestZones_Paged(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		w.Header().Set("Content-Type", "application/json")
		var result []map[string]any
		if page == 1 {
			result = []map[string]any{{"id": "z1", "name": "a.example.com"}}
		} else {
			result = []map[string]any{{"id": "z2", "name": "b.example.com"}}
		}
		resp := map[string]any{
			"success":     true,
			"errors":      []any{},
			"result":      result,
			"result_info": map[string]any{"total_pages": 2},
		}
		if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
			t.Errorf("encode: %v", encErr)
		}
	}))
	defer srv.Close()

	client := cloudflare.NewClientWithBase("tok", srv.URL, srv.Client())
	zones, err := client.Zones(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(zones) != 2 {
		t.Fatalf("want 2 zones across 2 pages, got %d", len(zones))
	}
}

func TestZones_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		resp := map[string]any{
			"success": false,
			"errors":  []map[string]any{{"message": "Invalid API token"}},
			"result":  []any{},
		}
		if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
			t.Errorf("encode: %v", encErr)
		}
	}))
	defer srv.Close()

	client := cloudflare.NewClientWithBase("bad", srv.URL, srv.Client())
	_, err := client.Zones(context.Background())
	if err == nil {
		t.Fatal("want error for API error response")
	}
	if err.Error() != "Invalid API token" {
		t.Errorf("want 'Invalid API token', got %q", err.Error())
	}
}

func TestRecords(t *testing.T) {
	srv := httptest.NewServer(apiResponse(t, []map[string]any{
		{"id": "r1", "type": "A", "name": "example.com", "content": "1.2.3.4", "ttl": float64(300), "proxied": true},
		{"id": "r2", "type": "CNAME", "name": "www.example.com", "content": "example.com", "ttl": float64(1), "proxied": false},
	}, 1))
	defer srv.Close()

	client := cloudflare.NewClientWithBase("tok", srv.URL, srv.Client())
	recs, err := client.Records(context.Background(), "zone1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(recs))
	}
	if recs[0].TTL != 300 || !recs[0].Proxied {
		t.Errorf("record 0 fields wrong: %+v", recs[0])
	}
	if recs[1].TTL != 1 || recs[1].Proxied {
		t.Errorf("record 1 fields wrong: %+v", recs[1])
	}
}

func TestRecords_MultipleErrorMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		resp := map[string]any{
			"success": false,
			"errors": []map[string]any{
				{"message": "err one"},
				{"message": "err two"},
			},
			"result": []any{},
		}
		if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
			t.Errorf("encode: %v", encErr)
		}
	}))
	defer srv.Close()

	client := cloudflare.NewClientWithBase("tok", srv.URL, srv.Client())
	_, err := client.Records(context.Background(), "z1")
	if err == nil {
		t.Fatal("want error")
	}
	if err.Error() != "err one; err two" {
		t.Errorf("want 'err one; err two', got %q", err.Error())
	}
}
