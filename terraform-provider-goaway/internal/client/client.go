package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	Endpoint string
	APIKey   string
	HTTP     *http.Client
}

func New(endpoint, apiKey string) *Client {
	return &Client{
		Endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		APIKey:   strings.TrimSpace(apiKey),
		HTTP:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.Endpoint+"/api"+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("api-key", c.APIKey)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("goaway API %s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

func (c *Client) AddUpstream(ctx context.Context, addr string) error {
	return c.do(ctx, http.MethodPost, "/upstream", map[string]string{"upstream": addr}, nil)
}

func (c *Client) DeleteUpstream(ctx context.Context, addr string) error {
	return c.do(ctx, http.MethodDelete, "/upstream?upstream="+url.QueryEscape(addr), nil, nil)
}

type Resolution struct {
	Domain string `json:"domain"`
	Value  string `json:"value"`
	Type   string `json:"type"`
}

func (c *Client) CreateResolution(ctx context.Context, r Resolution) error {
	if r.Type == "" {
		r.Type = "A"
	}
	return c.do(ctx, http.MethodPost, "/resolution", r, nil)
}

func (c *Client) DeleteResolution(ctx context.Context, domain, value string) error {
	q := url.Values{"domain": {domain}, "ip": {value}}
	return c.do(ctx, http.MethodDelete, "/resolution?"+q.Encode(), nil, nil)
}

func (c *Client) AddWhitelist(ctx context.Context, domain string) error {
	return c.do(ctx, http.MethodPost, "/whitelist", map[string]string{"domain": domain}, nil)
}

func (c *Client) DeleteWhitelist(ctx context.Context, domain string) error {
	return c.do(ctx, http.MethodDelete, "/whitelist?domain="+url.QueryEscape(domain), nil, nil)
}

type Group struct {
	ID                uint   `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	UseGlobalPolicies bool   `json:"useGlobalPolicies"`
}

func (c *Client) CreateGroup(ctx context.Context, g Group) (*Group, error) {
	var created Group
	err := c.do(ctx, http.MethodPost, "/groups", map[string]any{
		"name":              g.Name,
		"description":       g.Description,
		"useGlobalPolicies": g.UseGlobalPolicies,
	}, &created)
	return &created, err
}

func (c *Client) DeleteGroup(ctx context.Context, id uint) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/groups/%d", id), nil, nil)
}
