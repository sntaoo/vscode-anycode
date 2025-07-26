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

// ReferencesProvider 引用查找提供者
type ReferencesProvider struct {
	documents *documents.DocumentStore
	trees     *trees.Manager
	symbols   *symbols.Index
	languages *languages.Manager
}

// NewReferencesProvider 创建引用查找提供者
func NewReferencesProvider(docs *documents.DocumentStore, treesManager *trees.Manager,
	symbolIndex *symbols.Index, langManager *languages.Manager) *ReferencesProvider {
	return &ReferencesProvider{
		documents: docs,
		trees:     treesManager,
		symbols:   symbolIndex,
		languages: langManager,
	}
}

// Register 注册provider
func (p *ReferencesProvider) Register(server types.LSPServer) error {
	supportedLanguages := p.languages.GetSupportedLanguages("references",
		[]types.QueryType{types.QueryTypeReferences, types.QueryTypeIdentifiers})

	err := server.RegisterCapability("textDocument/references", supportedLanguages)
	if err != nil {
		return err
	}

	return server.RegisterHandler("textDocument/references", p.handleReferences)
}

// handleReferences 处理引用查找请求
func (p *ReferencesProvider) handleReferences(params *lsp.ReferenceParams) ([]lsp.Location, error) {
	doc, exists := p.documents.Get(params.TextDocument.URI)
	if !exists {
		return nil, nil
	}

	// 获取光标位置的标识符
	identifier, err := p.getIdentifierAtPosition(doc, params.Position)
	if err != nil || identifier == "" {
		return nil, err
	}

	var allReferences []lsp.Location

	// 查找当前文件中的引用
	localRefs, err := p.findLocalReferences(doc, identifier, params.Position, params.Context.IncludeDeclaration)
	if err == nil {
		allReferences = append(allReferences, localRefs...)
	}

	// 查找全局引用
	globalRefs, err := p.findGlobalReferences(doc, identifier, params.Context.IncludeDeclaration)
	if err == nil {
		for _, symLoc := range globalRefs {
			allReferences = append(allReferences, symLoc.Location)
		}
	}

	return allReferences, nil
}

// getIdentifierAtPosition 获取位置处的标识符
func (p *ReferencesProvider) getIdentifierAtPosition(doc *lsp.TextDocumentItem, position lsp.Position) (string, error) {
	tree, err := p.trees.GetParseTree(doc.URI)
	if err != nil || tree == nil {
		return "", err
	}

	languageID := p.languages.GetLanguageIDByURI(doc.URI)
	query, exists := p.languages.GetQuery(languageID, types.QueryTypeIdentifiers)
	if !exists {
		return "", nil
	}

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
				return string(node.Content([]byte(doc.Text))), nil
			}
		}
	}

	return "", nil
}

// findLocalReferences 查找当前文件中的引用
func (p *ReferencesProvider) findLocalReferences(doc *lsp.TextDocumentItem, identifier string, 
	position lsp.Position, includeDeclaration bool) ([]lsp.Location, error) {
	
	tree, err := p.trees.GetParseTree(doc.URI)
	if err != nil || tree == nil {
		return nil, err
	}

	languageID := p.languages.GetLanguageIDByURI(doc.URI)
	
	var results []lsp.Location

	// 使用references查询查找引用
	if query, exists := p.languages.GetQuery(languageID, types.QueryTypeReferences); exists {
		refs := p.findReferencesWithQuery(doc, tree, query, identifier)
		results = append(results, refs...)
	}

	// 如果没有专门的references查询，使用identifiers查询
	if len(results) == 0 {
		if query, exists := p.languages.GetQuery(languageID, types.QueryTypeIdentifiers); exists {
			refs := p.findReferencesWithQuery(doc, tree, query, identifier)
			results = append(results, refs...)
		}
	}

	// 如果不包括声明，过滤掉定义位置
	if !includeDeclaration {
		results = p.filterOutDeclarations(results, doc, identifier)
	}

	return results, nil
}

// findReferencesWithQuery 使用查询查找引用
func (p *ReferencesProvider) findReferencesWithQuery(doc *lsp.TextDocumentItem, tree *sitter.Tree, 
	query *sitter.Query, identifier string) []lsp.Location {
	
	var results []lsp.Location

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

	return results
}

// findGlobalReferences 查找全局引用
func (p *ReferencesProvider) findGlobalReferences(doc *lsp.TextDocumentItem, identifier string, 
	includeDeclaration bool) ([]*types.SymbolLocation, error) {
	
	// 从符号索引获取引用
	return p.symbols.GetReferences(identifier, doc)
}

// filterOutDeclarations 过滤掉声明位置
func (p *ReferencesProvider) filterOutDeclarations(locations []lsp.Location, doc *lsp.TextDocumentItem, 
	identifier string) []lsp.Location {
	
	tree, err := p.trees.GetParseTree(doc.URI)
	if err != nil || tree == nil {
		return locations
	}

	languageID := p.languages.GetLanguageIDByURI(doc.URI)
	
	// 获取所有定义位置
	var declarationLocations []lsp.Location
	
	if query, exists := p.languages.GetQuery(languageID, types.QueryTypeOutline); exists {
		declarationLocations = p.findDeclarationsWithQuery(doc, tree, query, identifier)
	}

	// 过滤结果
	var filtered []lsp.Location
	for _, loc := range locations {
		isDeclaration := false
		for _, decl := range declarationLocations {
			if p.locationsEqual(loc, decl) {
				isDeclaration = true
				break
			}
		}
		if !isDeclaration {
			filtered = append(filtered, loc)
		}
	}

	return filtered
}

// findDeclarationsWithQuery 使用查询查找声明
func (p *ReferencesProvider) findDeclarationsWithQuery(doc *lsp.TextDocumentItem, tree *sitter.Tree, 
	query *sitter.Query, identifier string) []lsp.Location {
	
	var results []lsp.Location

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

	return results
}

// isPositionInRange 检查位置是否在范围内
func (p *ReferencesProvider) isPositionInRange(pos lsp.Position, start, end sitter.Point) bool {
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

// locationsEqual 检查两个位置是否相等
func (p *ReferencesProvider) locationsEqual(a, b lsp.Location) bool {
	return a.URI == b.URI &&
		a.Range.Start.Line == b.Range.Start.Line &&
		a.Range.Start.Character == b.Range.Start.Character &&
		a.Range.End.Line == b.Range.End.Line &&
		a.Range.End.Character == b.Range.End.Character
}