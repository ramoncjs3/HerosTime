package game

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/httputil"
	"oldbeggar-refactor/internal/protocol"
)

type ServerResolver struct {
	http *httputil.Client
}

func NewServerResolver(httpClient *httputil.Client) *ServerResolver {
	return &ServerResolver{http: httpClient}
}

func (r *ServerResolver) RawNames(ctx context.Context, variant config.Variant) ([]string, error) {
	entries, err := r.fetch(ctx, variant)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if len(entry) < 6 {
			continue
		}
		name, _ := entry[5].(string)
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func (r *ServerResolver) Resolve(ctx context.Context, variant config.Variant) (map[string]ServerEndpoint, error) {
	entries, err := r.fetch(ctx, variant)
	if err != nil {
		return nil, err
	}
	out := make(map[string]ServerEndpoint)
	samples := make([]string, 0, 10)
	for _, raw := range entries {
		if len(raw) < 6 {
			continue
		}
		name, _ := raw[5].(string)
		if name != "" && len(samples) < 10 {
			samples = append(samples, name)
		}
		for _, rule := range variant.ServerRules {
			if !matchesPrefix(name, rule.MatchPrefix) {
				continue
			}
			zone := zoneNumber(name)
			if zone <= 0 {
				continue
			}
			code := fmt.Sprintf("%s%d", rule.CodePrefix, zone)
			out[code] = ServerEndpoint{
				Code:          code,
				QuickLoginURL: serverURL(raw, 2),
				GameURL:       serverURL(raw, 3),
				Name:          name,
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no servers matched variant %s rules; sample names: %s", variant.Name, strings.Join(samples, ", "))
	}
	return out, nil
}

func (r *ServerResolver) fetch(ctx context.Context, variant config.Variant) ([][]interface{}, error) {
	resp, err := r.http.Get(ctx, variant.ServerIndexURL)
	if err != nil {
		return nil, err
	}
	var serverIndex map[string][]string
	if err := json.Unmarshal(resp.Body, &serverIndex); err != nil {
		return nil, err
	}
	hosts := serverIndex["0"]
	if len(hosts) == 0 || hosts[0] == "" {
		return nil, fmt.Errorf("server index missing host 0")
	}

	listURL := serverListURL(hosts[0])
	listResp, err := r.http.Post(ctx, listURL, "application/x-www-form-urlencoded", []byte(variant.ServerListPayload), nil)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Code    int    `json:"code"`
		MsgData string `json:"MsgData"`
	}
	if err := json.Unmarshal(listResp.Body, &envelope); err != nil {
		return nil, err
	}
	if envelope.MsgData == "" {
		return nil, fmt.Errorf("server list returned empty MsgData, code=%d", envelope.Code)
	}
	if envelope.Code != 0 && envelope.Code != 200 {
		return nil, fmt.Errorf("server list returned code=%d", envelope.Code)
	}
	decoded, err := protocol.DecodeMsgData(envelope.MsgData)
	if err != nil {
		return nil, err
	}
	var payload struct {
		ServerObj map[string][]interface{} `json:"serverObj"`
	}
	if err := json.Unmarshal([]byte(decoded), &payload); err != nil {
		return nil, err
	}
	entries := make([][]interface{}, 0, len(payload.ServerObj))
	for _, entry := range payload.ServerObj {
		entries = append(entries, entry)
	}
	return entries, nil
}

func serverListURL(host string) string {
	host = strings.TrimRight(host, "/")
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host + ":9898/GetServerList"
	}
	return "http://" + host + ":9898/GetServerList"
}

func matchesPrefix(name, prefix string) bool {
	if prefix == "" {
		return false
	}
	return strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix))
}

func serverURL(entry []interface{}, pathIndex int) string {
	if len(entry) <= pathIndex {
		return ""
	}
	base, _ := entry[1].(string)
	path, _ := entry[pathIndex].(string)
	if base == "" || path == "" {
		return ""
	}
	base = strings.TrimRight(base, "/")
	path = strings.TrimLeft(path, "/")
	joined, err := url.JoinPath(base, path)
	if err != nil {
		return base + "/" + path
	}
	return joined
}

func zoneNumber(name string) int {
	current := 0
	last := 0
	inDigits := false
	for _, r := range name {
		if r >= '0' && r <= '9' {
			if !inDigits {
				current = 0
				inDigits = true
			}
			current = current*10 + int(r-'0')
			continue
		}
		if inDigits {
			last = current
			inDigits = false
		}
	}
	if inDigits {
		last = current
	}
	return last
}
