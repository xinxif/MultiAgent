// Package rag RAG 引擎：Loader 文档加载。
//
// 支持格式与实现取舍（依赖最小化）：
//   - txt/md/html：纯 Go 读文件 + HTML 去标签；
//   - pdf：github.com/dslipak/pdf 提取文本与页码（轻量但能力有限，可后续替换）；
//   - docx：docx 本质是 zip，标准库读 word/document.xml 去标签 —— 不引重型依赖。
package loader

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	pdf "github.com/dslipak/pdf"
)

// LoadedDoc 解析结果：分页内容（page 从 1 开始）。
type LoadedDoc struct {
	Pages []PageContent
}

// PageContent 单页/单段内容。
type PageContent struct {
	Page    int    // 页码（md/txt 无页码概念，按 1024 字节粗分段递增）
	Content string // 纯文本
}

// Load 按文件类型分发解析。
func Load(filename string, data []byte) (*LoadedDoc, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return loadPDF(data)
	case ".docx":
		return loadDocx(data)
	case ".html", ".htm":
		return loadPlainText(data), nil
	case ".md", ".markdown", ".txt", "": // 无后缀按文本处理
		return loadPlainText(data), nil
	default:
		return nil, fmt.Errorf("unsupported file type: %s", filepath.Ext(filename))
	}
}

// loadPlainText 纯文本：按 1024 字节粗分段模拟页码（保序，方便引用定位）。
func loadPlainText(data []byte) *LoadedDoc {
	text := normalizeText(string(data))
	var pages []PageContent
	const chunk = 1024
	for i, r := 0, []rune(text); i < len(r); i += chunk {
		end := i + chunk
		if end > len(r) {
			end = len(r)
		}
		pages = append(pages, PageContent{
			Page:    len(pages) + 1,
			Content: string(r[i:end]),
		})
	}
	if len(pages) == 0 {
		pages = append(pages, PageContent{Page: 1, Content: text})
	}
	return &LoadedDoc{Pages: pages}
}

// loadPDF 逐页提取文本。
func loadPDF(data []byte) (*LoadedDoc, error) {
	reader := bytes.NewReader(data)
	doc, err := pdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}

	var pages []PageContent
	for i := 0; i < doc.NumPage(); i++ {
		p := doc.Page(i + 1) // 页码从 1 开始
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue // 单页失败跳过，不阻塞整体
		}
		content := normalizeText(string(text))
		if strings.TrimSpace(content) == "" {
			continue
		}
		pages = append(pages, PageContent{Page: i + 1, Content: content})
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("no text extracted from pdf")
	}
	return &LoadedDoc{Pages: pages}, nil
}

// loadDocx 解压读 word/document.xml 去标签。
func loadDocx(data []byte) (*LoadedDoc, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open docx: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		xmlData, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}
		return loadPlainText([]byte(stripXML(string(xmlData)))), nil
	}
	return nil, fmt.Errorf("word/document.xml not found")
}

var xmlTagRe = regexp.MustCompile(`<[^>]+>`)

// stripXML 去 XML 标签（w:t 之间的文本自然保留）。
func stripXML(s string) string {
	return xmlTagRe.ReplaceAllString(s, " ")
}

// normalizeText 文本规范化：统一换行、去多余空白。
func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	// 压缩连续空行
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}
