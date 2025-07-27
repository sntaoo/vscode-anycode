package symbols

import (
	"log"
	"strings"
	"sync"

	"anycode-go-server/internal/documents"
	"anycode-go-server/internal/languages"
	"anycode-go-server/internal/storage"
	"anycode-go-server/internal/trees"
	"anycode-go-server/internal/types"

	"github.com/sourcegraph/go-lsp"
	sitter "github.com/smacker/go-tree-sitter"
)

// Index 符号索引
type Index struct {
	mu        sync.RWMutex
	storage   storage.SymbolInfoStorage
	documents *documents.DocumentStore
	trees     *trees.Manager
	languages *languages.Manager
	
	// 索引数据
	symbolMap map[string]map[string][]*types.SymbolLocation // symbol -> uri -> locations
	queue     map[string]bool // 待处理的文件队列
}

// NewIndex 创建符号索引
func NewIndex(storage storage.SymbolInfoStorage, docs *documents.DocumentStore, 
	treesManager *trees.Manager, langManager *languages.Manager) *Index {
	
	idx := &Index{
		storage:   storage,
		documents: docs,
		trees:     treesManager,
		languages: langManager,
		symbolMap: make(map[string]map[string][]*types.SymbolLocation),
		queue:     make(map[string]bool),
	}
	
	return idx
}

// AddFile 添加文件到索引
func (idx *Index) AddFile(uri string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	if isInteresting(uri) {
		idx.queue[uri] = true
	}
}

// RemoveFile 从索引中移除文件
func (idx *Index) RemoveFile(uri string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	// 从队列中移除
	delete(idx.queue, uri)
	
	// 从符号映射中移除
	for symbol, uriMap := range idx.symbolMap {
		delete(uriMap, uri)
		if len(uriMap) == 0 {
			delete(idx.symbolMap, symbol)
		}
	}
}

// ProcessFiles 处理文件队列
func (idx *Index) ProcessFiles(maxFiles int) error {
	idx.mu.Lock()
	files := make([]string, 0, maxFiles)
	count := 0
	for uri := range idx.queue {
		if count >= maxFiles {
			break
		}
		files = append(files, uri)
		delete(idx.queue, uri)
		count++
	}
	idx.mu.Unlock()
	
	// 处理文件
	for _, uri := range files {
		if err := idx.indexFile(uri); err != nil {
			log.Printf("Failed to index file %s: %v", uri, err)
		}
	}
	
	return nil
}

// indexFile 索引单个文件
func (idx *Index) indexFile(uri string) error {
	doc, exists := idx.documents.Get(uri)
	if !exists {
		return nil
	}
	
	// 获取解析树
	tree, err := idx.trees.GetParseTree(uri)
	if err != nil || tree == nil {
		return err
	}
	
	languageID := idx.languages.GetLanguageIDByURI(uri)
	
	// 提取符号
	symbols, err := idx.extractSymbols(doc, tree, languageID)
	if err != nil {
		return err
	}
	
	// 更新索引
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	// 清除旧的符号
	for symbol, uriMap := range idx.symbolMap {
		delete(uriMap, uri)
		if len(uriMap) == 0 {
			delete(idx.symbolMap, symbol)
		}
	}
	
	// 添加新符号
	for _, symLoc := range symbols {
		symbolName := getSymbolName(symLoc.Location, doc.Text)
		if symbolName == "" {
			continue
		}
		
		if idx.symbolMap[symbolName] == nil {
			idx.symbolMap[symbolName] = make(map[string][]*types.SymbolLocation)
		}
		
		idx.symbolMap[symbolName][uri] = append(idx.symbolMap[symbolName][uri], symLoc)
	}
	
	return nil
}

// extractSymbols 从文件中提取符号
func (idx *Index) extractSymbols(doc *lsp.TextDocumentItem, tree *sitter.Tree, languageID string) ([]*types.SymbolLocation, error) {
	var symbols []*types.SymbolLocation
	
	// 提取定义
	if query, exists := idx.languages.GetQuery(languageID, types.QueryTypeOutline); exists {
		defs := idx.executeQuery(query, tree, doc)
		symbols = append(symbols, defs...)
	}
	
	// 提取标识符
	if query, exists := idx.languages.GetQuery(languageID, types.QueryTypeIdentifiers); exists {
		idents := idx.executeQuery(query, tree, doc)
		symbols = append(symbols, idents...)
	}
	
	return symbols, nil
}

// executeQuery 执行查询
func (idx *Index) executeQuery(query *sitter.Query, tree *sitter.Tree, doc *lsp.TextDocumentItem) []*types.SymbolLocation {
	var results []*types.SymbolLocation
	
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	
	cursor.Exec(query, tree.RootNode())
	
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		
		for _, capture := range match.Captures {
			node := capture.Node
			
			// 创建位置信息
			startPoint := node.StartPoint()
			endPoint := node.EndPoint()
			
			location := lsp.Location{
				URI: doc.URI,
				Range: lsp.Range{
					Start: lsp.Position{
						Line:      int(startPoint.Row),
						Character: int(startPoint.Column),
					},
					End: lsp.Position{
						Line:      int(endPoint.Row),
						Character: int(endPoint.Column),
					},
				},
			}
			
			symbolLoc := &types.SymbolLocation{
				Location: location,
				Kind:     lsp.SKFunction, // 默认类型，实际应该根据查询结果确定
			}
			
			results = append(results, symbolLoc)
		}
	}
	
	return results
}

// GetDefinitions 获取符号定义
func (idx *Index) GetDefinitions(symbol string, fromDoc *lsp.TextDocumentItem) ([]*types.SymbolLocation, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	var results []*types.SymbolLocation
	
	// 查找匹配的符号
	for symName, uriMap := range idx.symbolMap {
		if strings.Contains(strings.ToLower(symName), strings.ToLower(symbol)) {
			for _, locations := range uriMap {
				results = append(results, locations...)
			}
		}
	}
	
	return results, nil
}

// GetReferences 获取符号引用
func (idx *Index) GetReferences(symbol string, fromDoc *lsp.TextDocumentItem) ([]*types.SymbolLocation, error) {
	// 引用和定义查找逻辑类似
	return idx.GetDefinitions(symbol, fromDoc)
}

// isInteresting 检查文件是否有趣（值得索引）
func isInteresting(uri string) bool {
	// 排除一些不需要索引的文件
	if strings.Contains(uri, "node_modules") ||
		strings.Contains(uri, ".git") ||
		strings.HasSuffix(uri, ".min.js") {
		return false
	}
	return true
}

// getSymbolName 从位置和文本中提取符号名称
func getSymbolName(location lsp.Location, text string) string {
	lines := strings.Split(text, "\n")
	if location.Range.Start.Line >= len(lines) {
		return ""
	}
	
	line := lines[location.Range.Start.Line]
	start := location.Range.Start.Character
	end := location.Range.End.Character
	
	if start >= len(line) || end > len(line) || start > end {
		return ""
	}
	
	return line[start:end]
}