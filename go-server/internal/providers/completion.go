package providers

import (
	"strings"

	"anycode-go-server/internal/documents"
	"anycode-go-server/internal/languages"
	"anycode-go-server/internal/symbols"
	"anycode-go-server/internal/trees"
	"anycode-go-server/internal/types"

	"github.com/sourcegraph/go-lsp"
	"github.com/smacker/go-tree-sitter"
)

// CompletionProvider 代码补全提供者
type CompletionProvider struct {
	documents *documents.DocumentStore
	trees     *trees.Manager
	symbols   *symbols.Index
	languages *languages.Manager
}

// NewCompletionProvider 创建代码补全提供者
func NewCompletionProvider(docs *documents.DocumentStore, treesManager *trees.Manager,
	symbolIndex *symbols.Index, langManager *languages.Manager) *CompletionProvider {
	return &CompletionProvider{
		documents: docs,
		trees:     treesManager,
		symbols:   symbolIndex,
		languages: langManager,
	}
}

// Register 注册provider
func (p *CompletionProvider) Register(server types.LSPServer) error {
	supportedLanguages := p.languages.GetSupportedLanguages("completions",
		[]types.QueryType{types.QueryTypeOutline, types.QueryTypeIdentifiers})

	err := server.RegisterCapability("textDocument/completion", supportedLanguages)
	if err != nil {
		return err
	}

	return server.RegisterHandler("textDocument/completion", p.handleCompletion)
}

// handleCompletion 处理补全请求
func (p *CompletionProvider) handleCompletion(params *lsp.CompletionParams) (*lsp.CompletionList, error) {
	doc, exists := p.documents.Get(params.TextDocument.URI)
	if !exists {
		return &lsp.CompletionList{}, nil
	}

	// 获取当前输入的前缀
	prefix, err := p.getCompletionPrefix(doc, params.Position)
	if err != nil {
		return &lsp.CompletionList{}, err
	}

	var items []lsp.CompletionItem

	// 获取本地符号补全
	localItems, err := p.getLocalCompletions(doc, prefix, params.Position)
	if err == nil {
		items = append(items, localItems...)
	}

	// 获取全局符号补全
	globalItems, err := p.getGlobalCompletions(doc, prefix)
	if err == nil {
		items = append(items, globalItems...)
	}

	// 获取关键字补全
	keywordItems := p.getKeywordCompletions(doc, prefix)
	items = append(items, keywordItems...)

	return &lsp.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil
}

// getCompletionPrefix 获取补全前缀
func (p *CompletionProvider) getCompletionPrefix(doc *lsp.TextDocumentItem, position lsp.Position) (string, error) {
	lines := strings.Split(doc.Text, "\n")
	if position.Line >= len(lines) {
		return "", nil
	}

	line := lines[position.Line]
	if position.Character > len(line) {
		position.Character = len(line)
	}

	// 向前查找标识符开始位置
	start := position.Character
	for start > 0 && isIdentifierChar(rune(line[start-1])) {
		start--
	}

	return line[start:position.Character], nil
}

// getLocalCompletions 获取本地符号补全
func (p *CompletionProvider) getLocalCompletions(doc *lsp.TextDocumentItem, prefix string, position lsp.Position) ([]lsp.CompletionItem, error) {
	tree, err := p.trees.GetParseTree(doc.URI)
	if err != nil || tree == nil {
		return nil, err
	}

	languageID := p.languages.GetLanguageIDByURI(doc.URI)
	query, exists := p.languages.GetQuery(languageID, types.QueryTypeLocals)
	if !exists {
		return nil, nil
	}

	var items []lsp.CompletionItem
	symbols := make(map[string]bool) // 去重

	// 执行查询获取局部符号
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
			symbolText := string(node.Content([]byte(doc.Text)))

			// 检查是否匹配前缀
			if strings.HasPrefix(strings.ToLower(symbolText), strings.ToLower(prefix)) && !symbols[symbolText] {
				symbols[symbolText] = true

				item := lsp.CompletionItem{
					Label:  symbolText,
					Kind:   lsp.CIKVariable,
					Detail: "Local symbol",
				}
				items = append(items, item)
			}
		}
	}

	return items, nil
}

// getGlobalCompletions 获取全局符号补全
func (p *CompletionProvider) getGlobalCompletions(doc *lsp.TextDocumentItem, prefix string) ([]lsp.CompletionItem, error) {
	if len(prefix) < 2 { // 避免返回太多结果
		return nil, nil
	}

	// 从符号索引获取匹配的符号
	symbolLocs, err := p.symbols.GetDefinitions(prefix, doc)
	if err != nil {
		return nil, err
	}

	var items []lsp.CompletionItem
	symbols := make(map[string]bool) // 去重

	for _, symLoc := range symbolLocs {
		symbolName := getSymbolNameFromLocation(symLoc.Location, doc.Text)
		if symbolName != "" && strings.HasPrefix(strings.ToLower(symbolName), strings.ToLower(prefix)) && !symbols[symbolName] {
			symbols[symbolName] = true

			kind := p.symbolKindToCompletionKind(symLoc.Kind)
			item := lsp.CompletionItem{
				Label:  symbolName,
				Kind:   kind,
				Detail: "Global symbol",
			}
			items = append(items, item)
		}
	}

	return items, nil
}

// getKeywordCompletions 获取关键字补全
func (p *CompletionProvider) getKeywordCompletions(doc *lsp.TextDocumentItem, prefix string) []lsp.CompletionItem {
	languageID := p.languages.GetLanguageIDByURI(doc.URI)
	keywords := getLanguageKeywords(languageID)

	var items []lsp.CompletionItem
	for _, keyword := range keywords {
		if strings.HasPrefix(strings.ToLower(keyword), strings.ToLower(prefix)) {
			item := lsp.CompletionItem{
				Label: keyword,
				Kind:  lsp.CIKKeyword,
			}
			items = append(items, item)
		}
	}

	return items
}

// symbolKindToCompletionKind 将符号类型转换为补全类型
func (p *CompletionProvider) symbolKindToCompletionKind(kind lsp.SymbolKind) lsp.CompletionItemKind {
	switch kind {
	case lsp.SKFile:
		return lsp.CIKFile
	case lsp.SKModule:
		return lsp.CIKModule
	case lsp.SKNamespace:
		return lsp.CIKModule
	case lsp.SKPackage:
		return lsp.CIKModule
	case lsp.SKClass:
		return lsp.CIKClass
	case lsp.SKMethod:
		return lsp.CIKMethod
	case lsp.SKProperty:
		return lsp.CIKProperty
	case lsp.SKField:
		return lsp.CIKField
	case lsp.SKConstructor:
		return lsp.CIKConstructor
	case lsp.SKEnum:
		return lsp.CIKEnum
	case lsp.SKInterface:
		return lsp.CIKInterface
	case lsp.SKFunction:
		return lsp.CIKFunction
	case lsp.SKVariable:
		return lsp.CIKVariable
	case lsp.SKConstant:
		return lsp.CIKConstant
	case lsp.SKString:
		return lsp.CIKValue
	case lsp.SKNumber:
		return lsp.CIKValue
	case lsp.SKBoolean:
		return lsp.CIKValue
	case lsp.SKArray:
		return lsp.CIKValue
	default:
		return lsp.CIKText
	}
}

// isIdentifierChar 检查字符是否是标识符字符
func isIdentifierChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// getSymbolNameFromLocation 从位置获取符号名称
func getSymbolNameFromLocation(location lsp.Location, text string) string {
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

// getLanguageKeywords 获取语言关键字
func getLanguageKeywords(languageID string) []string {
	switch languageID {
	case "javascript", "typescript":
		return []string{
			"abstract", "arguments", "await", "boolean", "break", "byte", "case", "catch",
			"char", "class", "const", "continue", "debugger", "default", "delete", "do",
			"double", "else", "enum", "eval", "export", "extends", "false", "final",
			"finally", "float", "for", "function", "goto", "if", "implements", "import",
			"in", "instanceof", "int", "interface", "let", "long", "native", "new",
			"null", "package", "private", "protected", "public", "return", "short",
			"static", "super", "switch", "synchronized", "this", "throw", "throws",
			"transient", "true", "try", "typeof", "var", "void", "volatile", "while", "with", "yield",
		}
	case "python":
		return []string{
			"False", "None", "True", "and", "as", "assert", "break", "class", "continue",
			"def", "del", "elif", "else", "except", "finally", "for", "from", "global",
			"if", "import", "in", "is", "lambda", "nonlocal", "not", "or", "pass",
			"raise", "return", "try", "while", "with", "yield",
		}
	case "go":
		return []string{
			"break", "case", "chan", "const", "continue", "default", "defer", "else",
			"fallthrough", "for", "func", "go", "goto", "if", "import", "interface",
			"map", "package", "range", "return", "select", "struct", "switch", "type", "var",
		}
	case "java":
		return []string{
			"abstract", "assert", "boolean", "break", "byte", "case", "catch", "char",
			"class", "const", "continue", "default", "do", "double", "else", "enum",
			"extends", "final", "finally", "float", "for", "goto", "if", "implements",
			"import", "instanceof", "int", "interface", "long", "native", "new", "package",
			"private", "protected", "public", "return", "short", "static", "strictfp",
			"super", "switch", "synchronized", "this", "throw", "throws", "transient",
			"try", "void", "volatile", "while",
		}
	default:
		return []string{}
	}
}