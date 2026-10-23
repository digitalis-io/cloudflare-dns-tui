package bdd_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cucumber/godog"

	"github.com/digitalis-io/cloudflare-dns-tui/internal/cloudflare"
)

type suite struct {
	srv     *httptest.Server
	client  *cloudflare.Client
	zones   []cloudflare.Zone
	records []cloudflare.DNSRecord
	err     error
	page    int
}

func (s *suite) cleanup() {
	if s.srv != nil {
		s.srv.Close()
		s.srv = nil
	}
	s.zones = nil
	s.records = nil
	s.err = nil
	s.page = 0
}

func (s *suite) theAPIReturnsZonesInArbitraryOrder() {
	s.srv = httptest.NewServer(jsonHandler(map[string]any{
		"success":     true,
		"errors":      []any{},
		"result":      []map[string]any{{"id": "z2", "name": "beta.example.com"}, {"id": "z1", "name": "alpha.example.com"}},
		"result_info": map[string]any{"total_pages": 1},
	}))
	s.client = cloudflare.NewClientWithBase("tok", s.srv.URL, s.srv.Client())
}

func (s *suite) noAPITokenIsSet() {
	s.client = cloudflare.NewClientWithBase("", "http://localhost", http.DefaultClient)
}

func (s *suite) theAPIHas2PagesOfZones() {
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.page++
		var result []map[string]any
		if s.page == 1 {
			result = []map[string]any{{"id": "z1", "name": "a.example.com"}}
		} else {
			result = []map[string]any{{"id": "z2", "name": "b.example.com"}}
		}
		writeJSON(w, map[string]any{
			"success":     true,
			"errors":      []any{},
			"result":      result,
			"result_info": map[string]any{"total_pages": 2},
		})
	}))
	s.client = cloudflare.NewClientWithBase("tok", s.srv.URL, s.srv.Client())
}

func (s *suite) theAPIReturnsAnAuthenticationError() {
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]any{
			"success": false,
			"errors":  []map[string]any{{"message": "Invalid API token"}},
			"result":  []any{},
		})
	}))
	s.client = cloudflare.NewClientWithBase("bad", s.srv.URL, s.srv.Client())
}

func (s *suite) theAPIReturnsHTTPWithNoErrorDetail(code int) {
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		writeJSON(w, map[string]any{"success": false, "errors": []any{}, "result": []any{}})
	}))
	s.client = cloudflare.NewClientWithBase("tok", s.srv.URL, s.srv.Client())
}

func (s *suite) theAPIReturnsTwoDNSRecords() {
	s.srv = httptest.NewServer(jsonHandler(map[string]any{
		"success":     true,
		"errors":      []any{},
		"result_info": map[string]any{"total_pages": 1},
		"result": []map[string]any{
			{"id": "r1", "type": "A", "name": "example.com", "content": "1.2.3.4", "ttl": float64(300), "proxied": true},
			{"id": "r2", "type": "CNAME", "name": "www.example.com", "content": "example.com", "ttl": float64(1), "proxied": false},
		},
	}))
	s.client = cloudflare.NewClientWithBase("tok", s.srv.URL, s.srv.Client())
}

func (s *suite) theAPIReturnsTwoErrorMessages(msg1, msg2 string) {
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		writeJSON(w, map[string]any{
			"success": false,
			"errors":  []map[string]any{{"message": msg1}, {"message": msg2}},
			"result":  []any{},
		})
	}))
	s.client = cloudflare.NewClientWithBase("tok", s.srv.URL, s.srv.Client())
}

func (s *suite) iFetchZones() {
	s.zones, s.err = s.client.Zones(context.Background())
}

func (s *suite) iFetchRecordsForZone(zoneID string) {
	s.records, s.err = s.client.Records(context.Background(), zoneID)
}

func (s *suite) theyAreReturnedSortedByNameAscending() error {
	if len(s.zones) == 0 {
		return fmt.Errorf("no zones returned")
	}
	for i := 1; i < len(s.zones); i++ {
		if s.zones[i-1].Name > s.zones[i].Name {
			return fmt.Errorf("zones not sorted: %q before %q", s.zones[i-1].Name, s.zones[i].Name)
		}
	}
	return nil
}

func (s *suite) iGetAnErrorContaining(sub string) error {
	if s.err == nil {
		return fmt.Errorf("want error containing %q, got nil", sub)
	}
	if !contains(s.err.Error(), sub) {
		return fmt.Errorf("want error containing %q, got %q", sub, s.err.Error())
	}
	return nil
}

func (s *suite) iReceiveZonesFromAllPages() error {
	if len(s.zones) != 2 {
		return fmt.Errorf("want 2 zones across 2 pages, got %d", len(s.zones))
	}
	return nil
}

func (s *suite) iReceive2Records() error {
	if len(s.records) != 2 {
		return fmt.Errorf("want 2 records, got %d", len(s.records))
	}
	return nil
}

func (s *suite) theFirstRecordHasProxiedAndTTL(proxied bool, ttl int) error {
	r := s.records[0]
	if r.Proxied != proxied || r.TTL != ttl {
		return fmt.Errorf("want proxied=%v ttl=%d, got proxied=%v ttl=%d", proxied, ttl, r.Proxied, r.TTL)
	}
	return nil
}

func (s *suite) theSecondRecordHasProxiedAndTTL(proxied bool, ttl int) error {
	r := s.records[1]
	if r.Proxied != proxied || r.TTL != ttl {
		return fmt.Errorf("want proxied=%v ttl=%d, got proxied=%v ttl=%d", proxied, ttl, r.Proxied, r.TTL)
	}
	return nil
}

func initScenario(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
	return ctx, nil
}

func TestBDD(t *testing.T) {
	s := &suite{}

	opts := godog.Options{
		Format: "pretty",
		Paths:  []string{"../features"},
	}

	suite := godog.TestSuite{
		Name: "cloudflare-client",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			sc.After(func(ctx context.Context, scn *godog.Scenario, err error) (context.Context, error) {
				s.cleanup()
				return ctx, nil
			})

			sc.Step(`^the API returns zones in arbitrary order$`, s.theAPIReturnsZonesInArbitraryOrder)
			sc.Step(`^no API token is set$`, s.noAPITokenIsSet)
			sc.Step(`^the API has 2 pages of zones$`, s.theAPIHas2PagesOfZones)
			sc.Step(`^the API returns an authentication error$`, s.theAPIReturnsAnAuthenticationError)
			sc.Step(`^the API returns HTTP (\d+) with no error detail$`, s.theAPIReturnsHTTPWithNoErrorDetail)
			sc.Step(`^the API returns two DNS records$`, s.theAPIReturnsTwoDNSRecords)
			sc.Step(`^the API returns two error messages "([^"]*)" and "([^"]*)"$`, s.theAPIReturnsTwoErrorMessages)

			sc.Step(`^I fetch zones$`, s.iFetchZones)
			sc.Step(`^I fetch records for zone "([^"]*)"$`, s.iFetchRecordsForZone)

			sc.Step(`^they are returned sorted by name ascending$`, s.theyAreReturnedSortedByNameAscending)
			sc.Step(`^I get an error containing "([^"]*)"$`, s.iGetAnErrorContaining)
			sc.Step(`^I receive zones from all pages$`, s.iReceiveZonesFromAllPages)
			sc.Step(`^I receive 2 records$`, s.iReceive2Records)
			sc.Step(`^the first record has proxied=true and TTL=(\d+)$`, func(ttl int) error {
				return s.theFirstRecordHasProxiedAndTTL(true, ttl)
			})
			sc.Step(`^the second record has proxied=false and TTL=(\d+)$`, func(ttl int) error {
				return s.theSecondRecordHasProxiedAndTTL(false, ttl)
			})
		},
		Options: &opts,
	}

	if status := suite.Run(); status != 0 {
		t.Fatalf("BDD scenarios failed with status %d", status)
	}
}

func jsonHandler(body map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, body)
	}
}

func writeJSON(w http.ResponseWriter, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
