package languages

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"anycode-go-server/internal/types"

	sitter "github.com/smacker/go-tree-sitter"
)

// Manager 语言管理器
type Manager struct {
	mu                   sync.RWMutex
	languages            map[string]*sitter.Language
	languageConfigs      map[string]types.LanguageConfig
	queries              map[string]map[types.QueryType]*sitter.Query
	languageByExtension  map[string]string
}

// NewManager 创建语言管理器
func NewManager() *Manager {
	return &Manager{
		languages:           make(map[string]*sitter.Language),
		languageConfigs:     make(map[string]types.LanguageConfig),
		queries:             make(map[string]map[types.QueryType]*sitter.Query),
		languageByExtension: make(map[string]string),
	}
}

// Initialize 初始化语言配置
func (m *Manager) Initialize(configs []types.LanguageConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, config := range configs {
		languageID := config.Info.LanguageID
		m.languageConfigs[languageID] = config
		
		// 构建扩展名映射
		for _, suffix := range config.Info.Suffixes {
			m.languageByExtension[suffix] = languageID
		}
		
		// 如果配置中包含查询，直接加载
		if config.Queries != nil && len(config.Queries) > 0 {
			m.loadQueriesFromConfig(languageID, config.Queries)
		}
	}
	
	return nil
}

// loadQueriesFromConfig 从配置中加载查询
func (m *Manager) loadQueriesFromConfig(languageID string, queries map[string]string) {
	// 如果还没有对应的语言，先创建空的查询映射
	if m.queries[languageID] == nil {
		m.queries[languageID] = make(map[types.QueryType]*sitter.Query)
	}
	
	// 获取语言实例，如果还没有则跳过（稍后SetLanguageData时会处理）
	language, exists := m.languages[languageID]
	if !exists {
		// 暂存查询内容，等语言加载后再处理
		return
	}
	
	// 加载查询
	for queryType, querySource := range queries {
		if querySource != "" {
			query, err := sitter.NewQuery([]byte(querySource), language)
			if err != nil {
				// 如果查询无效，创建空查询
				query, _ = sitter.NewQuery([]byte(""), language)
			}
			m.queries[languageID][types.QueryType(queryType)] = query
		}
	}
}

// SetLanguageData 设置语言数据
func (m *Manager) SetLanguageData(languageID string, data types.LanguageData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 解码语法数据
	grammarData, err := base64.StdEncoding.DecodeString(data.GrammarBase64)
	if err != nil {
		return fmt.Errorf("failed to decode grammar data: %w", err)
	}
	
	// 加载语言
	language := sitter.NewLanguage(grammarData)
	m.languages[languageID] = language
	
	// 加载查询
	queries := make(map[types.QueryType]*sitter.Query)
	for queryType, querySource := range data.Queries {
		if querySource != "" {
			query, err := sitter.NewQuery([]byte(querySource), language)
			if err != nil {
				// 如果查询无效，创建空查询
				query, _ = sitter.NewQuery([]byte(""), language)
			}
			queries[types.QueryType(queryType)] = query
		}
	}
	m.queries[languageID] = queries
	
	// 如果配置中有查询但之前没有语言，现在可以加载了
	if config, exists := m.languageConfigs[languageID]; exists && config.Queries != nil {
		m.loadQueriesFromConfigWithLanguage(languageID, config.Queries, language)
	}
	
	return nil
}

// loadQueriesFromConfigWithLanguage 使用指定语言从配置中加载查询
func (m *Manager) loadQueriesFromConfigWithLanguage(languageID string, queries map[string]string, language *sitter.Language) {
	if m.queries[languageID] == nil {
		m.queries[languageID] = make(map[types.QueryType]*sitter.Query)
	}
	
	// 加载查询，这些查询会覆盖或补充从LanguageData中加载的查询
	for queryType, querySource := range queries {
		if querySource != "" {
			query, err := sitter.NewQuery([]byte(querySource), language)
			if err != nil {
				// 如果查询无效，创建空查询
				query, _ = sitter.NewQuery([]byte(""), language)
			}
			m.queries[languageID][types.QueryType(queryType)] = query
		}
	}
}

// GetLanguage 获取语言
func (m *Manager) GetLanguage(languageID string) (*sitter.Language, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	language, exists := m.languages[languageID]
	return language, exists
}

// GetQuery 获取查询
func (m *Manager) GetQuery(languageID string, queryType types.QueryType) (*sitter.Query, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	langQueries, exists := m.queries[languageID]
	if !exists {
		return nil, false
	}
	
	query, exists := langQueries[queryType]
	return query, exists
}

// GetLanguageIDByURI 根据URI获取语言ID
func (m *Manager) GetLanguageIDByURI(uri string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 移除查询参数和片段
	cleanURI := uri
	if idx := strings.LastIndex(cleanURI, "?"); idx >= 0 {
		cleanURI = cleanURI[:idx]
	}
	if idx := strings.LastIndex(cleanURI, "#"); idx >= 0 {
		cleanURI = cleanURI[:idx]
	}
	
	// 获取文件扩展名
	ext := strings.TrimPrefix(filepath.Ext(cleanURI), ".")
	if ext == "" {
		return fmt.Sprintf("unknown/%s", uri)
	}
	
	if languageID, exists := m.languageByExtension[ext]; exists {
		return languageID
	}
	
	return fmt.Sprintf("unknown/%s", uri)
}

// GetSupportedLanguages 获取支持指定功能的语言
func (m *Manager) GetSupportedLanguages(feature string, queryTypes []types.QueryType) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var result []string
	for languageID, config := range m.languageConfigs {
		// 检查功能是否支持
		if !m.isFeatureSupported(config.Feature, feature) {
			continue
		}
		
		// 检查查询类型是否支持
		hasRequiredQuery := false
		for _, queryType := range queryTypes {
			if _, exists := config.Info.QueryInfo[string(queryType)]; exists {
				hasRequiredQuery = true
				break
			}
		}
		
		if hasRequiredQuery {
			result = append(result, languageID)
		}
	}
	
	return result
}

// isFeatureSupported 检查功能是否支持
func (m *Manager) isFeatureSupported(config types.FeatureConfig, feature string) bool {
	switch feature {
	case "completions":
		return config.Completions
	case "definitions":
		return config.Definitions
	case "references":
		return config.References
	case "highlights":
		return config.Highlights
	case "outline":
		return config.Outline
	case "folding":
		return config.Folding
	case "workspaceSymbols":
		return config.WorkspaceSymbols
	case "diagnostics":
		return config.Diagnostics
	default:
		return false
	}
}

// AllLanguageIDs 获取所有语言ID
func (m *Manager) AllLanguageIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var result []string
	for languageID := range m.languageConfigs {
		result = append(result, languageID)
	}
	return result
}