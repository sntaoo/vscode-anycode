package types

import (
	"github.com/sourcegraph/go-lsp"
	sitter "github.com/smacker/go-tree-sitter"
)

// InitOptions 初始化选项
type InitOptions struct {
	TreeSitterWasmURI    string                 `json:"treeSitterWasmUri"`
	SupportedLanguages   []LanguageConfig       `json:"supportedLanguages"`
	DatabaseName         string                 `json:"databaseName"`
}

// LanguageInfo 语言信息
type LanguageInfo struct {
	ExtensionID  string            `json:"extensionId"`
	LanguageID   string            `json:"languageId"`
	Suffixes     []string          `json:"suffixes"`
	SuppressedBy []string          `json:"suppressedBy"`
	QueryInfo    map[string]string `json:"queryInfo"`
}

// FeatureConfig 功能配置
type FeatureConfig struct {
	Completions      bool `json:"completions,omitempty"`
	Definitions      bool `json:"definitions,omitempty"`
	References       bool `json:"references,omitempty"`
	Highlights       bool `json:"highlights,omitempty"`
	Outline          bool `json:"outline,omitempty"`
	Folding          bool `json:"folding,omitempty"`
	WorkspaceSymbols bool `json:"workspaceSymbols,omitempty"`
	Diagnostics      bool `json:"diagnostics,omitempty"`
}

// LanguageConfig 语言配置
type LanguageConfig struct {
	Info    LanguageInfo  `json:"info"`
	Feature FeatureConfig `json:"feature"`
}

// LanguageData 语言数据
type LanguageData struct {
	GrammarBase64 string            `json:"grammarBase64"`
	Queries       map[string]string `json:"queries"`
}

// QueryType 查询类型
type QueryType string

const (
	QueryTypeOutline     QueryType = "outline"
	QueryTypeComments    QueryType = "comments"
	QueryTypeFolding     QueryType = "folding"
	QueryTypeLocals      QueryType = "locals"
	QueryTypeIdentifiers QueryType = "identifiers"
	QueryTypeReferences  QueryType = "references"
)

// ParseResult 解析结果
type ParseResult struct {
	Tree     *sitter.Tree
	Language *sitter.Language
}

// SymbolInfo 符号信息
type SymbolInfo struct {
	Definitions map[lsp.SymbolKind]bool `json:"definitions"`
	Usages      map[lsp.SymbolKind]bool `json:"usages"`
}

// SymbolLocation 符号位置
type SymbolLocation struct {
	Location lsp.Location   `json:"location"`
	Kind     lsp.SymbolKind `json:"kind"`
}

// DocumentVersion 文档版本
type DocumentVersion struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// Edit 编辑操作
type Edit struct {
	StartPosition lsp.Position `json:"startPosition"`
	OldEndPosition lsp.Position `json:"oldEndPosition"`
	NewEndPosition lsp.Position `json:"newEndPosition"`
	StartIndex     int          `json:"startIndex"`
	OldEndIndex    int          `json:"oldEndIndex"`
	NewEndIndex    int          `json:"newEndIndex"`
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Version int      `json:"version"`
	Tree    *sitter.Tree
	Edits   [][]Edit `json:"edits"`
}

// Provider 功能提供者接口
type Provider interface {
	Register(server LSPServer) error
}

// LSPServer LSP服务器接口
type LSPServer interface {
	RegisterHandler(method string, handler interface{}) error
	RegisterCapability(method string, selector []string) error
}