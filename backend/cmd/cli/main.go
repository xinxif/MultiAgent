// Agent CLI 入口：终端对话客户端。
//
// 设计取向：CLI 不直连数据库/网关，而是作为 HTTP 客户端调用 API 的 SSE 端点 ——
// 复用同一套鉴权、上下文组装与事件协议，证明"协议层与业务层彻底解耦"。
//
// 用法：
//
//	go run ./cmd/cli -addr http://127.0.0.1:8080 -user alice -pass 12345678
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	addr := flag.String("addr", "http://127.0.0.1:8080", "API 地址")
	user := flag.String("user", "", "用户名")
	pass := flag.String("pass", "", "密码")
	flag.Parse()
	if *user == "" || *pass == "" {
		fmt.Fprintln(os.Stderr, "需要 -user 与 -pass")
		os.Exit(1)
	}

	client := NewClient(*addr)
	if err := client.Login(*user, *pass); err != nil {
		fmt.Fprintln(os.Stderr, "登录失败:", err)
		os.Exit(1)
	}
	fmt.Println("已登录，输入消息开始对话（/quit 退出）：")

	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !sc.Scan() {
			break
		}
		text := strings.TrimSpace(sc.Text())
		switch {
		case text == "":
			continue
		case text == "/quit":
			return
		}
		if err := client.Chat(text); err != nil {
			fmt.Fprintln(os.Stderr, "对话失败:", err)
		}
	}
}

// Client API 客户端。
type Client struct {
	addr   string
	token  string
	agent  int64
	http   *http.Client
}

// NewClient 构造。
func NewClient(addr string) *Client {
	return &Client{addr: strings.TrimRight(addr, "/"), http: &http.Client{}}
}

// Login 登录保存 token。
func (c *Client) Login(user, pass string) error {
	body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	resp, err := c.http.Post(c.addr+"/api/v1/auth/login", "application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		Code int `json:"code"`
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if out.Code != 0 {
		return fmt.Errorf("%s", out.Message)
	}
	c.token = out.Data.AccessToken
	return nil
}

// Chat 发起一次 SSE 流式对话并渲染到终端。
func (c *Client) Chat(content string) error {
	body, _ := json.Marshal(map[string]any{"content": content, "agent_id": c.agent})
	req, err := http.NewRequest("POST", c.addr+"/api/v1/chat/stream", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, raw)
	}

	// 解析 SSE：event: 名 + data: JSON
	rd := bufio.NewReader(resp.Body)
	event := ""
	for {
		line, err := rd.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data := strings.TrimPrefix(line, "data: ")
			render(event, data)
		case line == "" && event != "":
			event = ""
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// render 事件渲染（按事件类型着色简化为纯文本标记）。
func render(event, data string) {
	var m map[string]any
	_ = json.Unmarshal([]byte(data), &m)
	switch event {
	case "delta":
		fmt.Print(m["content"])
	case "tool_call":
		fmt.Printf("\n🔧 调用工具 %v(%v)\n", m["name"], m["args"])
	case "tool_result":
		fmt.Printf("   ↳ %v\n", m["status"])
	case "agent_event":
		fmt.Printf("\n🤖 [%v] %v\n", m["type"], m["expert"])
	case "refs":
		fmt.Printf("\n📚 引用 %d 条资料\n", len(data))
	case "error":
		fmt.Printf("\n❌ %v\n", m["message"])
	case "done":
		fmt.Println()
	}
}
