// Package cloudflare provides a minimal Cloudflare API client for zones and DNS records.
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

// ErrMissingToken is returned when no API token was supplied.
var ErrMissingToken = errors.New("API token is required")

const defaultBase = "https://api.cloudflare.com/client/v4"

// Zone represents a Cloudflare DNS zone.
type Zone struct {
	ID   string
	Name string
}

// DNSRecord represents a single DNS record within a zone.
type DNSRecord struct {
	ID      string
	Type    string
	Name    string
	Content string
	TTL     int // 1 = automatic
	Proxied bool
}

// Client is a Cloudflare API client.
type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewClient creates a Client with the given API token.
func NewClient(token string) (*Client, error) {
	if token == "" {
		return nil, ErrMissingToken
	}
	return &Client{
		token:   token,
		baseURL: defaultBase,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// NewClientWithBase creates a Client with a custom base URL (for testing).
func NewClientWithBase(token, baseURL string, hc *http.Client) *Client {
	return &Client{token: token, baseURL: baseURL, http: hc}
}

// Zones returns all zones sorted by name.
func (c *Client) Zones(ctx context.Context) ([]Zone, error) {
	items, err := c.paged(ctx, "/zones", nil)
	if err != nil {
		return nil, err
	}
	zones := make([]Zone, 0, len(items))
	for _, j := range items {
		zones = append(zones, Zone{
			ID:   stringField(j, "id"),
			Name: stringField(j, "name"),
		})
	}
	sort.Slice(zones, func(i, k int) bool { return zones[i].Name < zones[k].Name })
	return zones, nil
}

// Records returns all DNS records for the given zone ID.
func (c *Client) Records(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	items, err := c.paged(ctx, fmt.Sprintf("/zones/%s/dns_records", zoneID), nil)
	if err != nil {
		return nil, err
	}
	recs := make([]DNSRecord, 0, len(items))
	for _, j := range items {
		recs = append(recs, recordFromMap(j))
	}
	return recs, nil
}

// CreateRecord creates a new DNS record in the given zone.
func (c *Client) CreateRecord(ctx context.Context, zoneID string, rec DNSRecord) (DNSRecord, error) {
	body, err := json.Marshal(recordToMap(rec))
	if err != nil {
		return DNSRecord{}, fmt.Errorf("marshal: %w", err)
	}
	result, err := c.mutate(ctx, http.MethodPost, fmt.Sprintf("/zones/%s/dns_records", zoneID), body)
	if err != nil {
		return DNSRecord{}, err
	}
	return recordFromMap(result), nil
}

// UpdateRecord replaces a DNS record.
func (c *Client) UpdateRecord(ctx context.Context, zoneID string, rec DNSRecord) error {
	body, err := json.Marshal(recordToMap(rec))
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	_, err = c.mutate(ctx, http.MethodPatch, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, rec.ID), body)
	return err
}

// DeleteRecord removes a DNS record by ID.
func (c *Client) DeleteRecord(ctx context.Context, zoneID, recordID string) error {
	_, err := c.mutate(ctx, http.MethodDelete, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID), nil)
	return err
}

func (c *Client) mutate(ctx context.Context, method, path string, body []byte) (map[string]any, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		Success bool           `json:"success"`
		Errors  []apiError     `json:"errors"`
		Result  map[string]any `json:"result"`
	}
	if decErr := json.NewDecoder(resp.Body).Decode(&envelope); decErr != nil {
		return nil, fmt.Errorf("decode: %w", decErr)
	}
	if !envelope.Success {
		if len(envelope.Errors) > 0 {
			return nil, fmt.Errorf("%s", envelope.Errors[0].Message)
		}
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return envelope.Result, nil
}

func recordFromMap(m map[string]any) DNSRecord {
	ttl := 1
	if v, ok := m["ttl"].(float64); ok {
		ttl = int(v)
	}
	proxied := false
	if v, ok := m["proxied"].(bool); ok {
		proxied = v
	}
	return DNSRecord{
		ID:      stringField(m, "id"),
		Type:    stringField(m, "type"),
		Name:    stringField(m, "name"),
		Content: stringField(m, "content"),
		TTL:     ttl,
		Proxied: proxied,
	}
}

func recordToMap(rec DNSRecord) map[string]any {
	return map[string]any{
		"type":    rec.Type,
		"name":    rec.Name,
		"content": rec.Content,
		"ttl":     rec.TTL,
		"proxied": rec.Proxied,
	}
}

func (c *Client) paged(ctx context.Context, path string, query map[string]string) ([]map[string]any, error) {
	var out []map[string]any
	for page := 1; ; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		q := req.URL.Query()
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("per_page", "100")
		for k, v := range query {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Authorization", "Bearer "+c.token)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		var body struct {
			Success    bool             `json:"success"`
			Errors     []apiError       `json:"errors"`
			Result     []map[string]any `json:"result"`
			ResultInfo *resultInfo      `json:"result_info"`
		}
		if decErr := json.NewDecoder(resp.Body).Decode(&body); decErr != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d: decode: %w", resp.StatusCode, decErr)
		}
		_ = resp.Body.Close()

		if !body.Success {
			if len(body.Errors) > 0 {
				msgs := make([]string, 0, len(body.Errors))
				for _, e := range body.Errors {
					msgs = append(msgs, e.Message)
				}
				return nil, fmt.Errorf("%s", joinStrings(msgs, "; "))
			}
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		out = append(out, body.Result...)

		if body.ResultInfo == nil || page >= body.ResultInfo.TotalPages {
			break
		}
	}
	return out, nil
}

type apiError struct {
	Message string `json:"message"`
}

type resultInfo struct {
	TotalPages int `json:"total_pages"`
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for _, s := range ss[1:] {
		out += sep + s
	}
	return out
}
