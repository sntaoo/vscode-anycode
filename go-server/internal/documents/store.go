package documents

import (
	"sync"

	"github.com/sourcegraph/go-lsp"
)

// DocumentStore 文档存储
type DocumentStore struct {
	mu        sync.RWMutex
	documents map[string]*lsp.TextDocumentItem
	
	// 事件回调
	onDidOpen    []func(*lsp.DidOpenTextDocumentParams)
	onDidChange  []func(*lsp.DidChangeTextDocumentParams)
	onDidClose   []func(*lsp.DidCloseTextDocumentParams)
}

// NewDocumentStore 创建文档存储
func NewDocumentStore() *DocumentStore {
	return &DocumentStore{
		documents: make(map[string]*lsp.TextDocumentItem),
	}
}

// OnDidOpen 注册文档打开事件
func (ds *DocumentStore) OnDidOpen(callback func(*lsp.DidOpenTextDocumentParams)) {
	ds.onDidOpen = append(ds.onDidOpen, callback)
}

// OnDidChange 注册文档变更事件
func (ds *DocumentStore) OnDidChange(callback func(*lsp.DidChangeTextDocumentParams)) {
	ds.onDidChange = append(ds.onDidChange, callback)
}

// OnDidClose 注册文档关闭事件
func (ds *DocumentStore) OnDidClose(callback func(*lsp.DidCloseTextDocumentParams)) {
	ds.onDidClose = append(ds.onDidClose, callback)
}

// DidOpen 处理文档打开
func (ds *DocumentStore) DidOpen(params *lsp.DidOpenTextDocumentParams) {
	ds.mu.Lock()
	ds.documents[params.TextDocument.URI] = &params.TextDocument
	ds.mu.Unlock()
	
	// 触发事件
	for _, callback := range ds.onDidOpen {
		callback(params)
	}
}

// DidChange 处理文档变更
func (ds *DocumentStore) DidChange(params *lsp.DidChangeTextDocumentParams) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	
	doc, exists := ds.documents[params.TextDocument.URI]
	if !exists {
		return
	}
	
	// 应用变更
	for _, change := range params.ContentChanges {
		if change.Range == nil {
			// 全文替换
			doc.Text = change.Text
		} else {
			// 增量变更 - 简化实现，实际应该根据range进行精确替换
			doc.Text = change.Text
		}
	}
	
	doc.Version = params.TextDocument.Version
	
	// 触发事件
	for _, callback := range ds.onDidChange {
		callback(params)
	}
}

// DidClose 处理文档关闭
func (ds *DocumentStore) DidClose(params *lsp.DidCloseTextDocumentParams) {
	ds.mu.Lock()
	delete(ds.documents, params.TextDocument.URI)
	ds.mu.Unlock()
	
	// 触发事件
	for _, callback := range ds.onDidClose {
		callback(params)
	}
}

// Get 获取文档
func (ds *DocumentStore) Get(uri string) (*lsp.TextDocumentItem, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	doc, exists := ds.documents[uri]
	return doc, exists
}

// All 获取所有文档
func (ds *DocumentStore) All() map[string]*lsp.TextDocumentItem {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	
	result := make(map[string]*lsp.TextDocumentItem)
	for uri, doc := range ds.documents {
		result[uri] = doc
	}
	return result
}

// Remove 移除文档
func (ds *DocumentStore) Remove(uri string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.documents, uri)
}