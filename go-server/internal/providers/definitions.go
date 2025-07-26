package providers

import (
	"anycode-go-server/internal/documents"
	"anycode-go-server/internal/languages"
	"anycode-go-server/internal/symbols"
	"anycode-go-server/internal/trees"
	"anycode-go-server/internal/types"

	"github.com/sourcegraph/go-lsp"
	sitter "github.com/smacker/go-tree-sitter"
)

// DefinitionProvider 定义跳转提供者
type DefinitionProvider struct {
	documents *documents.DocumentStore
	trees     *trees.Manager
	symbols   *symbols.Index
	languages *languages.Manager
}

// NewDefinitionProvider 创建定义跳转提供者
func NewDefinitionProvider(docs *documents.DocumentStore, treesManager *trees.Manager, 
	symbolIndex *symbols.Index, langManager *languages.Manager) *DefinitionProvider {
	return &DefinitionProvider{
		documents: docs,
		trees:     treesManager,
		symbols:   symbolIndex,
		languages: langManager,
	}
}

// Register 注册provider
func (p *DefinitionProvider) Register(server types.LSPServer) error {
	supportedLanguages := p.languages.GetSupportedLanguages("definitions", 
		[]types.QueryType{types.QueryTypeLocals, types.QueryTypeOutline})
	
	err := server.RegisterCapability("textDocument/definition", supportedLanguages)
	if err != nil {
		return err
	}
	
	return server.RegisterHandler("textDocument/definition", p.handleDefinition)
}

// handleDefinition 处理定义跳转请求
func (p *DefinitionProvider) handleDefinition(params *lsp.TextDocumentPositionParams) ([]lsp.Location, error) {
	doc, exists := p.documents.Get(params.TextDocument.URI)
	if !exists {
		return nil, nil
	}
	
	// 尝试在当前文件中查找本地定义
	localDefs, err := p.findLocalDefinitions(doc, params.Position)
	if err == nil && len(localDefs) > 0 {
		return localDefs, nil
	}
	
	// 全局查找
	globalDefs, err := p.findGlobalDefinitions(doc, params.Position)
	if err != nil {
		return nil, err
	}
	
	var results []lsp.Location
	for _, symLoc := range globalDefs {
		results = append(results, symLoc.Location)
	}
	
	return results, nil
}

// findLocalDefinitions 查找局部定义
func (p *DefinitionProvider) findLocalDefinitions(doc *lsp.TextDocumentItem, position lsp.Position) ([]lsp.Location, error) {
	tree, err := p.trees.GetParseTree(doc.URI)
	if err != nil || tree == nil {
		return nil, err
	}
	
	languageID := p.languages.GetLanguageIDByURI(doc.URI)
	
	// 使用locals查询查找局部定义
	query, exists := p.languages.GetQuery(languageID, types.QueryTypeLocals)
	if !exists {
		return nil, nil
	}
	
	// 找到光标位置的节点
	node := tree.RootNode().NamedDescendantForPointRange(
		sitter.Point{Row: uint32(position.Line), Column: uint32(position.Character)},
		sitter.Point{Row: uint32(position.Line), Column: uint32(position.Character)},
	)
	
	if node == nil {
		return nil, nil
	}
	
	// 获取标识符文本
	identifierText := node.Content([]byte(doc.Text))
	if len(identifierText) == 0 {
		return nil, nil
	}
	
	// 在当前作用域中查找定义
	return p.findDefinitionsInScope(doc, tree, query, string(identifierText), node)
}

// findDefinitionsInScope 在作用域中查找定义
func (p *DefinitionProvider) findDefinitionsInScope(doc *lsp.TextDocumentItem, tree *sitter.Tree, 
	query *sitter.Query, identifier string, contextNode *sitter.Node) ([]lsp.Location, error) {
	
	var results []lsp.Location
	
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	
	// 查找包含上下文节点的函数或类
	scope := p.findEnclosingScope(contextNode)
	if scope == nil {
		scope = tree.RootNode()
	}
	
	cursor.Exec(query, scope)
	
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		
		for _, capture := range match.Captures {
			node := capture.Node
			nodeText := string(node.Content([]byte(doc.Text)))
			
			if nodeText == identifier {
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
				
				results = append(results, location)
			}
		}
	}
	
	return results, nil
}

// findEnclosingScope 查找包围的作用域
func (p *DefinitionProvider) findEnclosingScope(node *sitter.Node) *sitter.Node {
	current := node.Parent()
	for current != nil {
		nodeType := current.Type()
		// 检查是否是函数、类或其他作用域节点
		if nodeType == "function_declaration" || 
		   nodeType == "method_definition" ||
		   nodeType == "class_declaration" ||
		   nodeType == "block_statement" {
			return current
		}
		current = current.Parent()
	}
	return nil
}

// findGlobalDefinitions 查找全局定义
func (p *DefinitionProvider) findGlobalDefinitions(doc *lsp.TextDocumentItem, position lsp.Position) ([]*types.SymbolLocation, error) {
	tree, err := p.trees.GetParseTree(doc.URI)
	if err != nil || tree == nil {
		return nil, err
	}
	
	languageID := p.languages.GetLanguageIDByURI(doc.URI)
	query, exists := p.languages.GetQuery(languageID, types.QueryTypeIdentifiers)
	if !exists {
		return nil, nil
	}
	
	// 找到光标位置的标识符
	identifier := p.getIdentifierAtPosition(tree, query, position, doc.Text)
	if identifier == "" {
		return nil, nil
	}
	
	// 在符号索引中查找
	return p.symbols.GetDefinitions(identifier, doc)
}

// getIdentifierAtPosition 获取位置处的标识符
func (p *DefinitionProvider) getIdentifierAtPosition(tree *sitter.Tree, query *sitter.Query, 
	position lsp.Position, text string) string {
	
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
			startPoint := node.StartPoint()
			endPoint := node.EndPoint()
			
			// 检查位置是否在节点范围内
			if p.isPositionInRange(position, startPoint, endPoint) {
				return string(node.Content([]byte(text)))
			}
		}
	}
	
	return ""
}

// isPositionInRange 检查位置是否在范围内
func (p *DefinitionProvider) isPositionInRange(pos lsp.Position, start, end sitter.Point) bool {
	if pos.Line < int(start.Row) || pos.Line > int(end.Row) {
		return false
	}
	
	if pos.Line == int(start.Row) && pos.Character < int(start.Column) {
		return false
	}
	
	if pos.Line == int(end.Row) && pos.Character > int(end.Column) {
		return false
	}
	
	return true
}