package trees

import (
	"sync"

	"anycode-go-server/internal/documents"
	"anycode-go-server/internal/languages"
	"anycode-go-server/internal/types"

	"github.com/sourcegraph/go-lsp"
	sitter "github.com/smacker/go-tree-sitter"
)

// Manager tree-sitter解析器管理器
type Manager struct {
	mu        sync.RWMutex
	cache     map[string]*types.CacheEntry
	maxCacheSize int
	documents *documents.DocumentStore
	languages *languages.Manager
	parser    *sitter.Parser
}

// NewManager 创建解析器管理器
func NewManager(docStore *documents.DocumentStore, langManager *languages.Manager) *Manager {
	m := &Manager{
		cache:     make(map[string]*types.CacheEntry),
		maxCacheSize: 100,
		documents: docStore,
		languages: langManager,
		parser:    sitter.NewParser(),
	}
	
	// 监听文档变更事件
	docStore.OnDidChange(m.onDocumentChange)
	
	return m
}

// onDocumentChange 处理文档变更
func (m *Manager) onDocumentChange(params *lsp.DidChangeTextDocumentParams) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	uri := params.TextDocument.URI
	entry, exists := m.cache[uri]
	if !exists {
		return
	}
	
	// 为每个变更创建编辑信息
	var edits []types.Edit
	for _, change := range params.ContentChanges {
		if change.Range != nil {
			edit := types.Edit{
				StartPosition:  change.Range.Start,
				OldEndPosition: change.Range.End,
				// NewEndPosition 需要计算
				StartIndex:     0, // 需要根据position计算
				OldEndIndex:    0, // 需要根据range计算
				NewEndIndex:    0, // 需要根据新文本长度计算
			}
			edits = append(edits, edit)
		}
	}
	
	// 将编辑添加到缓存条目
	entry.Edits = append(entry.Edits, edits)
}

// GetParseTree 获取解析树
func (m *Manager) GetParseTree(uri string) (*sitter.Tree, error) {
	doc, exists := m.documents.Get(uri)
	if !exists {
		return nil, nil
	}
	
	languageID := m.languages.GetLanguageIDByURI(uri)
	language, exists := m.languages.GetLanguage(languageID)
	if !exists {
		return nil, nil
	}
	
	return m.parseWithCache(doc, language)
}

// parseWithCache 使用缓存进行解析
func (m *Manager) parseWithCache(doc *lsp.TextDocumentItem, language *sitter.Language) (*sitter.Tree, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	uri := doc.URI
	entry, exists := m.cache[uri]
	
	// 如果缓存存在且版本匹配，直接返回
	if exists && entry.Version == doc.Version {
		return entry.Tree, nil
	}
	
	// 设置解析器语言和超时
	m.parser.SetLanguage(language)
	m.parser.SetTimeoutMicros(1000 * 1000) // 1秒超时
	
	var tree *sitter.Tree
	var err error
	
	if !exists {
		// 第一次解析，直接解析全文
		tree, err = m.parser.ParseString(nil, doc.Text)
		if err != nil {
			return nil, err
		}
		
		entry = &types.CacheEntry{
			Version: doc.Version,
			Tree:    tree,
			Edits:   make([][]types.Edit, 0),
		}
		m.cache[uri] = entry
	} else {
		// 增量解析
		oldTree := entry.Tree
		
		// 应用所有编辑
		for _, editGroup := range entry.Edits {
			for _, edit := range editGroup {
				oldTree.Edit(sitter.EditInput{
					StartByte:    uint32(edit.StartIndex),
					OldEndByte:   uint32(edit.OldEndIndex),
					NewEndByte:   uint32(edit.NewEndIndex),
					StartPoint: sitter.Point{
						Row:    uint32(edit.StartPosition.Line),
						Column: uint32(edit.StartPosition.Character),
					},
					OldEndPoint: sitter.Point{
						Row:    uint32(edit.OldEndPosition.Line),
						Column: uint32(edit.OldEndPosition.Character),
					},
					NewEndPoint: sitter.Point{
						Row:    uint32(edit.NewEndPosition.Line),
						Column: uint32(edit.NewEndPosition.Character),
					},
				})
			}
		}
		
		// 基于旧树进行增量解析
		tree, err = m.parser.ParseString(oldTree, doc.Text)
		if err != nil {
			return nil, err
		}
		
		// 更新缓存条目
		entry.Version = doc.Version
		entry.Tree = tree
		entry.Edits = make([][]types.Edit, 0)
	}
	
	// 检查缓存大小，如果超过限制则清理
	if len(m.cache) > m.maxCacheSize {
		m.cleanCache()
	}
	
	return tree, nil
}

// cleanCache 清理缓存
func (m *Manager) cleanCache() {
	// 简单的LRU实现：删除一半的条目
	count := 0
	target := len(m.cache) / 2
	
	for uri, entry := range m.cache {
		if count >= target {
			break
		}
		entry.Tree.Close()
		delete(m.cache, uri)
		count++
	}
}

// RemoveFromCache 从缓存中移除
func (m *Manager) RemoveFromCache(uri string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if entry, exists := m.cache[uri]; exists {
		entry.Tree.Close()
		delete(m.cache, uri)
	}
}

// Close 关闭管理器
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 关闭所有缓存的树
	for _, entry := range m.cache {
		entry.Tree.Close()
	}
	m.cache = make(map[string]*types.CacheEntry)
	
	// 关闭解析器
	m.parser.Close()
}