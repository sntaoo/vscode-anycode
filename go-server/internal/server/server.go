package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"anycode-go-server/internal/documents"
	"anycode-go-server/internal/languages"
	"anycode-go-server/internal/providers"
	"anycode-go-server/internal/storage"
	"anycode-go-server/internal/symbols"
	"anycode-go-server/internal/trees"
	"anycode-go-server/internal/types"

	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"
)

// Server LSP服务器
type Server struct {
	conn        *jsonrpc2.Conn
	factory     storage.StorageFactory
	
	// 核心组件
	documents   *documents.DocumentStore
	languages   *languages.Manager
	trees       *trees.Manager
	symbols     *symbols.Index
	storage     storage.SymbolInfoStorage
	
	// Provider
	providers   []types.Provider
	
	// 处理器映射
	handlers    map[string]interface{}
	
	// 能力映射
	capabilities map[string][]string
}

// NewServer 创建新的服务器
func NewServer(factory storage.StorageFactory) *Server {
	return &Server{
		factory:      factory,
		handlers:     make(map[string]interface{}),
		capabilities: make(map[string][]string),
	}
}

// Handle 实现jsonrpc2.Handler接口
func (s *Server) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	s.conn = conn
	
	switch req.Method {
	case "initialize":
		s.handleInitialize(ctx, req)
	case "initialized":
		s.handleInitialized(ctx, req)
	case "textDocument/didOpen":
		s.handleDidOpen(ctx, req)
	case "textDocument/didChange":
		s.handleDidChange(ctx, req)
	case "textDocument/didClose":
		s.handleDidClose(ctx, req)
	case "workspace/didChangeWatchedFiles":
		s.handleDidChangeWatchedFiles(ctx, req)
	default:
		// 检查是否有注册的处理器
		if handler, exists := s.handlers[req.Method]; exists {
			s.handleWithRegisteredHandler(ctx, req, handler)
		} else {
			// 未知方法
			conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
				Code:    jsonrpc2.CodeMethodNotFound,
				Message: fmt.Sprintf("method not found: %s", req.Method),
			})
		}
	}
}

// handleInitialize 处理初始化请求
func (s *Server) handleInitialize(ctx context.Context, req *jsonrpc2.Request) {
	var params lsp.InitializeParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		s.conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: err.Error(),
		})
		return
	}
	
	// 解析初始化选项
	var initOptions types.InitOptions
	if params.InitializationOptions != nil {
		if err := json.Unmarshal(*params.InitializationOptions, &initOptions); err != nil {
			log.Printf("Failed to parse initialization options: %v", err)
		}
	}
	
	// 初始化组件
	if err := s.initializeComponents(initOptions); err != nil {
		s.conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: err.Error(),
		})
		return
	}
	
	// 创建响应
	result := lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: &lsp.TextDocumentSyncOptionsOrKind{
				Options: &lsp.TextDocumentSyncOptions{
					OpenClose: true,
					Change:    lsp.TDSKIncremental,
				},
			},
		},
	}
	
	s.conn.Reply(ctx, req.ID, result)
}

// initializeComponents 初始化所有组件
func (s *Server) initializeComponents(initOptions types.InitOptions) error {
	// 创建存储
	var err error
	s.storage, err = s.factory.Create(initOptions.DatabaseName)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	
	// 初始化组件
	s.documents = documents.NewDocumentStore()
	s.languages = languages.NewManager()
	s.trees = trees.NewManager(s.documents, s.languages)
	s.symbols = symbols.NewIndex(s.storage, s.documents, s.trees, s.languages)
	
	// 初始化语言配置
	if err := s.languages.Initialize(initOptions.SupportedLanguages); err != nil {
		return fmt.Errorf("failed to initialize languages: %w", err)
	}
	
	// 创建providers
	s.providers = []types.Provider{
		providers.NewDefinitionProvider(s.documents, s.trees, s.symbols, s.languages),
		providers.NewCompletionProvider(s.documents, s.trees, s.symbols, s.languages),
		providers.NewReferencesProvider(s.documents, s.trees, s.symbols, s.languages),
	}
	
	// 设置文档事件处理
	s.setupDocumentEventHandlers()
	
	return nil
}

// setupDocumentEventHandlers 设置文档事件处理器
func (s *Server) setupDocumentEventHandlers() {
	// 监听文档打开事件
	s.documents.OnDidOpen(func(params *lsp.DidOpenTextDocumentParams) {
		s.symbols.AddFile(params.TextDocument.URI)
	})
	
	// 监听文档变更事件
	s.documents.OnDidChange(func(params *lsp.DidChangeTextDocumentParams) {
		s.symbols.AddFile(params.TextDocument.URI)
	})
	
	// 监听文档关闭事件
	s.documents.OnDidClose(func(params *lsp.DidCloseTextDocumentParams) {
		s.trees.RemoveFromCache(params.TextDocument.URI)
	})
}

// handleInitialized 处理初始化完成通知
func (s *Server) handleInitialized(ctx context.Context, req *jsonrpc2.Request) {
	// 注册所有providers
	for _, provider := range s.providers {
		if err := provider.Register(s); err != nil {
			log.Printf("Failed to register provider: %v", err)
		}
	}
	
	// 发送动态注册请求
	s.registerCapabilities(ctx)
}

// registerCapabilities 注册能力
func (s *Server) registerCapabilities(ctx context.Context) {
	for method, selector := range s.capabilities {
		registration := lsp.Registration{
			ID:     method,
			Method: method,
			RegisterOptions: map[string]interface{}{
				"documentSelector": s.createDocumentSelector(selector),
			},
		}
		
		params := lsp.RegistrationParams{
			Registrations: []lsp.Registration{registration},
		}
		
		s.conn.Notify(ctx, "client/registerCapability", params)
	}
}

// createDocumentSelector 创建文档选择器
func (s *Server) createDocumentSelector(languages []string) []lsp.DocumentFilter {
	var filters []lsp.DocumentFilter
	for _, lang := range languages {
		filters = append(filters, lsp.DocumentFilter{
			Language: lang,
		})
	}
	return filters
}

// handleDidOpen 处理文档打开
func (s *Server) handleDidOpen(ctx context.Context, req *jsonrpc2.Request) {
	var params lsp.DidOpenTextDocumentParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return
	}
	
	s.documents.DidOpen(&params)
}

// handleDidChange 处理文档变更
func (s *Server) handleDidChange(ctx context.Context, req *jsonrpc2.Request) {
	var params lsp.DidChangeTextDocumentParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return
	}
	
	s.documents.DidChange(&params)
}

// handleDidClose 处理文档关闭
func (s *Server) handleDidClose(ctx context.Context, req *jsonrpc2.Request) {
	var params lsp.DidCloseTextDocumentParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return
	}
	
	s.documents.DidClose(&params)
}

// handleDidChangeWatchedFiles 处理监视文件变更
func (s *Server) handleDidChangeWatchedFiles(ctx context.Context, req *jsonrpc2.Request) {
	var params lsp.DidChangeWatchedFilesParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return
	}
	
	for _, change := range params.Changes {
		switch change.Type {
		case lsp.WKTCreated:
			s.symbols.AddFile(change.URI)
		case lsp.WKTChanged:
			s.symbols.AddFile(change.URI)
		case lsp.WKTDeleted:
			s.symbols.RemoveFile(change.URI)
			s.documents.Remove(change.URI)
			s.trees.RemoveFromCache(change.URI)
		}
	}
}

// handleWithRegisteredHandler 使用注册的处理器处理请求
func (s *Server) handleWithRegisteredHandler(ctx context.Context, req *jsonrpc2.Request, handler interface{}) {
	// 使用反射调用处理器 - 这里简化实现
	// 实际应该根据处理器类型进行适当的类型转换和调用
	switch h := handler.(type) {
	case func(*lsp.TextDocumentPositionParams) ([]lsp.Location, error):
		var params lsp.TextDocumentPositionParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			s.conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInvalidParams,
				Message: err.Error(),
			})
			return
		}
		
		result, err := h(&params)
		if err != nil {
			s.conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: err.Error(),
			})
			return
		}
		
		s.conn.Reply(ctx, req.ID, result)
		
	case func(*lsp.CompletionParams) (*lsp.CompletionList, error):
		var params lsp.CompletionParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			s.conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInvalidParams,
				Message: err.Error(),
			})
			return
		}
		
		result, err := h(&params)
		if err != nil {
			s.conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: err.Error(),
			})
			return
		}
		
		s.conn.Reply(ctx, req.ID, result)
		
	case func(*lsp.ReferenceParams) ([]lsp.Location, error):
		var params lsp.ReferenceParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			s.conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInvalidParams,
				Message: err.Error(),
			})
			return
		}
		
		result, err := h(&params)
		if err != nil {
			s.conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: err.Error(),
			})
			return
		}
		
		s.conn.Reply(ctx, req.ID, result)
	default:
		s.conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: "unsupported handler type",
		})
	}
}

// RegisterHandler 实现LSPServer接口 - 注册处理器
func (s *Server) RegisterHandler(method string, handler interface{}) error {
	s.handlers[method] = handler
	return nil
}

// RegisterCapability 实现LSPServer接口 - 注册能力
func (s *Server) RegisterCapability(method string, selector []string) error {
	s.capabilities[method] = selector
	return nil
}

// SetLanguageData 设置语言数据
func (s *Server) SetLanguageData(languageID string, data types.LanguageData) error {
	return s.languages.SetLanguageData(languageID, data)
}

// ProcessSymbolIndex 处理符号索引
func (s *Server) ProcessSymbolIndex(maxFiles int) error {
	return s.symbols.ProcessFiles(maxFiles)
}

// Close 关闭服务器
func (s *Server) Close() error {
	if s.trees != nil {
		s.trees.Close()
	}
	
	if s.storage != nil && s.factory != nil {
		return s.factory.Destroy(s.storage)
	}
	
	return nil
}