// OpenAI 兼容协议实现：覆盖 DeepSeek / 通义 / 智谱 / Kimi / 豆包 Ark / Ollama。
//
// 实现要点：
//  1. Chat Completions 的消息协议：assistant 的 tool_calls 如何回传、tool 消息如何接续；
//  2. SSE 流式解析：delta 增量拼接、tool_calls 按 index 聚合片段、[DONE] 终止帧；
//  3. SSE 解析直接手写，便于精确控制增量拼接与终止帧处理。
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// openaiCompatibleProvider OpenAI 兼容协议实现。
type openaiCompatibleProvider struct {
	providerCode string //! 模型的名称
	httpClient   *http.Client
}

func (p *openaiCompatibleProvider) client() *http.Client {
	if p.httpClient == nil {
		p.httpClient = &http.Client{Timeout: 0} // 超时由 ctx 控制
		/*
			&http.Client{Timeout: 0}   // 完全等价于
			&http.Client{}             // 也等价于
			http.DefaultClient         // （默认客户端也是无超时）
		*/
	}
	return p.httpClient
}

// ---- 请求结构 ----

type openaiTool struct {
	Type     string       `json:"type"` //!种类标签，目前固定 "function"
	Function openaiFnSpec `json:"function"`
}

// !Specification（规格/定义/说明）
type openaiFnSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"` //! 可选，空则省略 "description"
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openaiRequest struct {
	Model         string         `json:"model"`
	Messages      []openaiMsg    `json:"messages"`              //!对话历史（system/user/assistant/tool 消息）
	Tools         []openaiTool   `json:"tools,omitempty"`       //!可用工具列表（函数定义）
	ToolChoice    string         `json:"tool_choice,omitempty"` //!工具选择策略
	Temperature   float32        `json:"temperature,omitempty"`
	TopP          float32        `json:"top_p,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	Stream        bool           `json:"stream"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"` //!流式的附加选项（如要 usage）
}

/*
"auto"	模型自主判断：需要工具就调，不需要就直接回文本（默认值）
"none"	禁止调用工具，只回文本
"required"	强制必须调至少一个工具，不许只回文本
{"type":"function","function":{"name":"get_weather"}}	强制调指定那一个
*/
type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openaiMsg struct {
	Role             string       `json:"role"`
	Content          any          `json:"content"`                     // string 或 null（有 tool_calls 时）
	ReasoningContent string       `json:"reasoning_content,omitempty"` //!思考内容 不是 OpenAI 标准字段，是供应商扩展
	ToolCalls        []openaiCall `json:"tool_calls,omitempty"`
	ToolCallID       string       `json:"tool_call_id,omitempty"`
}

type openaiCall struct {
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

/*
	{
	  "role": "assistant",
	  "content": null,
	  "tool_calls": [
	    {
	      "id": "call_abc",
	      "type": "function",
	      "function": { "name": "get_weather", "arguments": "{\"location\":\"北京\"}" }
	    },
	    {
	      "id": "call_def",
	      "type": "function",
	      "function": { "name": "get_exchange_rate", "arguments": "{\"from\":\"USD\",\"to\":\"CNY\"}" }
	    }
	  ]
	}
*/

/*
流式 tool_calls 需要 index 才能聚合：ID 仅在第一帧出现，后续碎片帧的 id 均为空，
只能依靠 index 判断参数片段属于哪一个调用。
// 第 1 帧：id/name 都在
{"tool_calls":[{"index":0, "id":"call_abc", "type":"function",
                "function":{"name":"get_weather","arguments":""}}]}

// 第 2 帧：无 id，仅 index
{"tool_calls":[{"index":0, "function":{"arguments":"{\"loc"}}]}

// 第 3 帧：仍无 id
{"tool_calls":[{"index":0, "function":{"arguments":"ation\":\"北京\"}"}}]}
*/

// ---- 响应结构 ----

type openaiResp struct {
	ID      string `json:"id"` //!本次 completion 的 id。这次响应（completion）的唯一标识符"*，OpenAI 生成的形如 chatcmpl-abc123... 的字符串
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content          string       `json:"content"`
			ReasoningContent string       `json:"reasoning_content"`
			ToolCalls        []openaiCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"` //!"stop" 模型正常说完了。"tool_calls" 模型要调用工具。"length" 撞到 max_tokens 上限被截断
	} `json:"choices"`
	Usage *openaiUsage `json:"usage"`
	Error *openaiErr   `json:"error"`
}

// 一次请求可能返回多个候选续写（choices）。默认 n=1，数组内只有一个元素，
// 因此上层代码统一取 resp.Choices[0]；n>1 会导致 token 费用按倍数增长，
// 且流式模式下多路交织难以支持，多数供应商会忽略或报错。
type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openaiErr struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// ---- 内部消息转换 ----

func toOpenAIMsgs(msgs []Message) []openaiMsg {
	out := make([]openaiMsg, 0, len(msgs))
	for _, m := range msgs {
		om := openaiMsg{Role: m.Role}
		switch {
		case m.Role == RoleAssistant && len(m.ToolCalls) > 0:
			// 有工具调用的 assistant 消息：content 允许为空
			om.Content = m.Content
			for _, tc := range m.ToolCalls {
				om.ToolCalls = append(om.ToolCalls, openaiCall{
					ID: tc.ID, Type: "function",
					Function: struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					}{Name: tc.Name, Arguments: tc.ArgsJSON},
				})
			}
		case m.Role == RoleTool:
			om.Content = m.Content
			om.ToolCallID = m.ToolCallID
		default:
			om.Content = m.Content
			om.ReasoningContent = m.ReasoningContent
		}
		out = append(out, om)
	}
	return out
}

func toOpenAITools(defs []ToolDef) []openaiTool {
	if len(defs) == 0 {
		return nil
	}
	out := make([]openaiTool, 0, len(defs))
	for _, d := range defs {
		out = append(out, openaiTool{
			Type: "function",
			Function: openaiFnSpec{
				Name: d.Name, Description: d.Description, Parameters: d.Parameters,
			},
		})
	}
	return out
}

// ---- 非流式 ----
func (p *openaiCompatibleProvider) Chat(ctx context.Context, desc *ModelDescriptor, req *ChatRequest) (*ChatResponse, error) {
	body := openaiRequest{
		Model: desc.ModelName, Messages: toOpenAIMsgs(req.Messages),
		Tools: toOpenAITools(req.Tools), Temperature: desc.Temperature,
		TopP: desc.TopP, MaxTokens: desc.MaxTokens,
	}

	if len(body.Tools) > 0 { //!只有在传了工具定义时才设置
		body.ToolChoice = "auto"
	}

	httpReq, err := p.newHTTPRequest(ctx, desc, body)
	if err != nil {
		return nil, err
	}

	//!发送http请求
	httpResp, err := p.client().Do(httpReq)
	if err != nil {
		return nil, classifyHTTPError(err)
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, parseAPIError(httpResp.StatusCode, raw)
	}

	var resp openaiResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("upstream: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices")
	}

	ch := resp.Choices[0]
	calls := make([]ToolCall, 0, len(ch.Message.ToolCalls))
	for _, c := range ch.Message.ToolCalls {
		calls = append(calls, ToolCall{ID: c.ID, Name: c.Function.Name, ArgsJSON: c.Function.Arguments})
	}

	usage := Usage{}
	if resp.Usage != nil {
		usage = Usage{PromptTokens: resp.Usage.PromptTokens, CompletionTokens: resp.Usage.CompletionTokens}
	}
	return &ChatResponse{
		Content: ch.Message.Content, ReasoningContent: ch.Message.ReasoningContent,
		ToolCalls: calls, FinishReason: ch.FinishReason, Model: desc.ModelName, Usage: usage,
	}, nil
}

// newHTTPRequest 构造上游请求。
func (p *openaiCompatibleProvider) newHTTPRequest(ctx context.Context, desc *ModelDescriptor, body any) (*http.Request, error) {
	payload, err := json.Marshal(body) //!Go 对象到JSON 字节
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	url := strings.TrimRight(desc.BaseURL, "/") + "/chat/completions"

	//!只能发字节，NewReader 只是给 payload 套了个"从哪读到哪"的游标（零拷贝）
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if desc.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+desc.APIKey)
	}
	return req, nil
}

// ---- 流式 ----
// !它用来在多个 goroutine / 多层函数调用之间传递三类信息
// 某处调用 cancel()，所有持有该 ctx 的地方都能感知到"该停了"
// context.WithTimeout，到点自动取消
// context.WithValue，常用来传 trace id、认证信息等
func (p *openaiCompatibleProvider) ChatStream(ctx context.Context, desc *ModelDescriptor, req *ChatRequest) (<-chan StreamEvent, error) {
	body := openaiRequest{
		Model: desc.ModelName, Messages: toOpenAIMsgs(req.Messages),
		Tools: toOpenAITools(req.Tools), Temperature: desc.Temperature,
		TopP: desc.TopP, MaxTokens: desc.MaxTokens,
		Stream: true, StreamOptions: &streamOptions{IncludeUsage: true},
	}
	if len(body.Tools) > 0 {
		body.ToolChoice = "auto"
	}

	httpReq, err := p.newHTTPRequest(ctx, desc, body)
	if err != nil {
		return nil, err
	}

	httpResp, err := p.client().Do(httpReq)
	if err != nil {
		return nil, classifyHTTPError(err)
	}
	if httpResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, parseAPIError(httpResp.StatusCode, raw)
	}

	// 事件通道：带缓冲减少阻塞；ctx 取消时 goroutine 退出
	events := make(chan StreamEvent, 64)
	//!返回协程对象，数据持续给协程对象中写数据
	go p.consumeStream(ctx, httpResp.Body, events)
	return events, nil
}

// consumeStream 解析 SSE 数据流。
//
//	SSE 帧格式：
//	  data: {"choices":[{"delta":{"content":"x"},...}]}
//	  data: [DONE]
//
//	tool_calls 的 arguments 是分片到达的（按 index 聚合后拼 JSON）。
func (p *openaiCompatibleProvider) consumeStream(ctx context.Context, body io.ReadCloser, events chan<- StreamEvent) {
	//! 流的本质是一个不关闭的HTTP响应体
	defer close(events)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	//!初始容量64KB,单个 token 最大 512KB
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)

	type callAgg struct {
		id, name, args strings.Builder
	}

	aggs := map[int]*callAgg{}
	finish := ""
	var usage *Usage
	model := ""

	//匿名函数
	emit := func(ev StreamEvent) {
		select {
		case events <- ev:
		case <-ctx.Done(): // 上游取消，停止读取
		}
	}
	/*
		OpenAI 的流式响应是 SSE (Server-Sent Events) 格式，一个"事件"由一个或多个字段行组成：
		data: {"choices":[{"delta":{"content":"你"}}]}      ← 数据行，以 data: 开头
															← 空行，分隔两个事件
		data: {"choices":[{"delta":{"content":"好"}}]}      ← 又一个数据行

		: keep-alive                                        ← 注释行，以 : 开头（心跳）
		data: [DONE]                                        ← 结束标记
	*/
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue // 忽略空行与注释行
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var frame struct {
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Content          string       `json:"content"`
					ReasoningContent string       `json:"reasoning_content"`
					ToolCalls        []openaiCall `json:"tool_calls"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *openaiUsage `json:"usage"`
		}
		//!json转为go对象
		if err := json.Unmarshal([]byte(data), &frame); err != nil {
			continue // 跳过无法解析的帧（部分供应商会发注释帧）
		}
		if frame.Model != "" {
			model = frame.Model
		}

		for _, ch := range frame.Choices {
			d := ch.Delta
			if d.ReasoningContent != "" {
				emit(StreamEvent{ReasoningContent: d.ReasoningContent, Model: model})
			}
			if d.Content != "" {
				emit(StreamEvent{Content: d.Content, Model: model})
			}
			/*
				模型调用 get_weather 且参数为 {"location":"北京"} 时，参数会分多帧到达
				（d 为每帧的 delta）：
				// 第 1 帧 delta
				{"tool_calls":[{
				   "index":0,
				   "id":"call_abc123",
				   "type":"function",
				   "function":{"name":"get_weather","arguments":""}
				}]}

				// 第 2 帧 delta
				{"tool_calls":[{
				   "index":0,
				   "function":{"arguments":"{\"loc"}
				}]}

				// 第 3 帧 delta
				{"tool_calls":[{
				   "index":0,
				   "function":{"arguments":"ation\":\"北京\"}"}
				}]}
			*/
			for _, tc := range d.ToolCalls {
				agg := aggs[tc.Index] //! "index":0,"id":"call_abc123"
				if agg == nil {       //第一次遇到这个工具调用
					agg = &callAgg{}
					aggs[tc.Index] = agg
				}
				if tc.ID != "" {
					agg.id.WriteString(tc.ID)
				}
				if tc.Function.Name != "" {
					agg.name.WriteString(tc.Function.Name)
				}
				agg.args.WriteString(tc.Function.Arguments)
			}
			if ch.FinishReason != nil && *ch.FinishReason != "" {
				finish = *ch.FinishReason
			}
		}
		if frame.Usage != nil {
			usage = &Usage{PromptTokens: frame.Usage.PromptTokens, CompletionTokens: frame.Usage.CompletionTokens}
		}
	}

	// 终帧：携带聚合完成的工具调用与用量
	calls := make([]ToolCall, 0, len(aggs))
	for i := 0; i < len(aggs); i++ {
		if a, ok := aggs[i]; ok {
			calls = append(calls, ToolCall{ID: a.id.String(), Name: a.name.String(), ArgsJSON: a.args.String()})
		}
	}
	if len(calls) > 0 {
		finish = "tool_calls"
	}
	emit(StreamEvent{ToolCalls: calls, FinishReason: finish, Usage: usage, Model: model})
}

// ---- Embedding ----

func (p *openaiCompatibleProvider) Embed(ctx context.Context, desc *ModelDescriptor, texts []string) ([][]float32, error) {
	payload, _ := json.Marshal(map[string]any{"model": desc.ModelName, "input": texts})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(desc.BaseURL, "/")+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if desc.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+desc.APIKey)
	}

	resp, err := p.client().Do(req)
	if err != nil {
		return nil, classifyHTTPError(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp.StatusCode, raw)
	}

	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode embeddings: %w", err)
	}
	vectors := make([][]float32, len(texts))
	for _, d := range out.Data {
		vectors[d.Index] = d.Embedding
	}
	return vectors, nil
}

// ---- Rerank（Jina/SiliconFlow 风格 /rerank 接口）----

type rerankResp struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float32 `json:"relevance_score"`
	} `json:"results"`
}

func (p *openaiCompatibleProvider) Rerank(ctx context.Context, desc *ModelDescriptor, query string, docs []string, topN int) ([]RerankResult, error) {
	payload, _ := json.Marshal(map[string]any{
		"model": desc.ModelName, "query": query, "documents": docs, "top_n": topN,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(desc.BaseURL, "/")+"/rerank", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if desc.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+desc.APIKey)
	}

	resp, err := p.client().Do(req)
	if err != nil {
		return nil, classifyHTTPError(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp.StatusCode, raw)
	}
	var out rerankResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode rerank: %w", err)
	}
	results := make([]RerankResult, 0, len(out.Results))
	for _, r := range out.Results {
		results = append(results, RerankResult{Index: r.Index, Score: r.RelevanceScore})
	}
	return results, nil
}

// ---- 错误分类（决定重试/熔断策略）----

type retryableError struct{ err error }

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

// IsRetryable 是否可重试（429/5xx/网络错误）。
func IsRetryable(err error) bool {
	var re *retryableError
	return asRetryable(err, &re)
}

func asRetryable(err error, target **retryableError) bool {
	for err != nil {
		if re, ok := err.(*retryableError); ok {
			*target = re
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func classifyHTTPError(err error) error {
	return &retryableError{err: err}
}

// parseAPIError 上游非 200：429/5xx 标记可重试。
func parseAPIError(status int, body []byte) error {
	msg := string(body)
	var eResp openaiErr
	if json.Unmarshal(body, &eResp) == nil && eResp.Message != "" {
		msg = eResp.Message
	}
	err := fmt.Errorf("upstream %d: %s", status, msg)
	if status == http.StatusTooManyRequests || status >= 500 {
		return &retryableError{err: err}
	}
	return err
}

// defaultFirstTokenTimeout 首 token 超时兜底（网关可覆盖）。
const defaultFirstTokenTimeout = 30 * time.Second
