// 内置工具：网络搜索与 HTTP 请求。
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// SearchConfig 搜索工具配置。
type SearchConfig struct {
	SearxngURL string
	HTTPClient *http.Client
	// DomainWhitelist http_client 工具的域名白名单（后缀匹配）
	DomainWhitelist []string
}

// RegisterWeb 注册 web_search 与 http_client。
func RegisterWeb(r Registry, cfg SearchConfig) {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	r.Register("web_search", func(ctx context.Context, args map[string]any, _ string) (string, error) {
		return runWebSearch(ctx, cfg, args)
	})
	r.Register("http_client", func(ctx context.Context, args map[string]any, _ string) (string, error) {
		return runHTTPClient(ctx, cfg, args)
	})
}

// runWebSearch 通过 SearXNG 搜索（自托管，免 API Key；可替换 Tavily 等）。
func runWebSearch(ctx context.Context, cfg SearchConfig, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	count := 5
	if v, ok := args["count"].(float64); ok && int(v) > 0 && int(v) <= 20 {
		count = int(v)
	}

	params := url.Values{
		"q":     {query},
		"format": {"json"},
		"language": {"zh-CN"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(cfg.SearxngURL, "/")+"?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}

	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", err
	}

	var sr struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &sr); err != nil {
		return "", fmt.Errorf("parse search result: %v", err)
	}

	// 只保留前 N 条并结构化输出（模型可读）
	out := make([]map[string]string, 0, count)
	for i, r := range sr.Results {
		if i >= count {
			break
		}
		out = append(out, map[string]string{
			"title": r.Title, "url": r.URL, "snippet": truncateText(r.Content, 200),
		})
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

// runHTTPClient 白名单域名的 GET 请求。
func runHTTPClient(ctx context.Context, cfg SearchConfig, args map[string]any) (string, error) {
	rawURL, _ := args["url"].(string)
	if rawURL == "" {
		return "", fmt.Errorf("url is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("invalid url")
	}
	// 白名单校验（SSRF 防护的第一层）
	if !domainAllowed(u.Hostname(), cfg.DomainWhitelist) {
		return "", fmt.Errorf("domain %s not in whitelist", u.Hostname())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	if h, ok := args["headers"].(map[string]any); ok {
		for k, v := range h {
			req.Header.Set(k, fmt.Sprintf("%v", v))
		}
	}

	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))

	// 粗略截取正文（去 HTML 标签），给模型的应该是"内容"而不是原始 HTML
	text := stripHTML(string(body))
	return fmt.Sprintf(`{"status": %d, "content": %q}`, resp.StatusCode, truncateText(text, 4000)), nil
}

// domainAllowed 后缀匹配白名单。
func domainAllowed(host string, whitelist []string) bool {
	for _, w := range whitelist {
		if host == w || strings.HasSuffix(host, "."+w) {
			return true
		}
	}
	return false
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>\s*`)

// stripHTML 粗提取 HTML 正文。
func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// truncateText 文本截断。
func truncateText(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
