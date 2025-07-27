package languages

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"anycode-go-server/internal/types"
)

// PackageConfig 语言包配置
type PackageConfig struct {
	Name        string `json:"name"`
	Publisher   string `json:"publisher"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	License     string `json:"license"`
	Version     string `json:"version"`
	Contributes struct {
		AnycodeLanguages AnycodeLanguageConfig `json:"anycodeLanguages"`
	} `json:"contributes"`
}

// AnycodeLanguageConfig anycode语言配置
type AnycodeLanguageConfig struct {
	GrammarPath  string            `json:"grammarPath"`
	LanguageID   string            `json:"languageId"`
	Extensions   []string          `json:"extensions"`
	QueryPaths   map[string]string `json:"queryPaths"`
	SuppressedBy []string          `json:"suppressedBy"`
}

// PackageLoader 语言包加载器
type PackageLoader struct {
	packagesDir string
}

// NewPackageLoader 创建语言包加载器
func NewPackageLoader(packagesDir string) *PackageLoader {
	return &PackageLoader{
		packagesDir: packagesDir,
	}
}

// LoadAllPackages 加载所有语言包
func (loader *PackageLoader) LoadAllPackages() ([]types.LanguageConfig, error) {
	var configs []types.LanguageConfig

	// 扫描语言包目录
	entries, err := os.ReadDir(loader.packagesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read packages directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		packageName := entry.Name()
		if !strings.HasPrefix(packageName, "anycode-") {
			continue
		}

		config, err := loader.loadPackage(packageName)
		if err != nil {
			fmt.Printf("Warning: failed to load package %s: %v\n", packageName, err)
			continue
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// loadPackage 加载单个语言包
func (loader *PackageLoader) loadPackage(packageName string) (types.LanguageConfig, error) {
	packageDir := filepath.Join(loader.packagesDir, packageName)

	// 读取package.json
	packageConfig, err := loader.readPackageJSON(packageDir)
	if err != nil {
		return types.LanguageConfig{}, err
	}

	// 读取查询文件
	queries, err := loader.loadQueries(packageDir, packageConfig.Contributes.AnycodeLanguages.QueryPaths)
	if err != nil {
		return types.LanguageConfig{}, err
	}

	// 构建语言信息
	langInfo := types.LanguageInfo{
		ExtensionID:  packageConfig.Name,
		LanguageID:   packageConfig.Contributes.AnycodeLanguages.LanguageID,
		Suffixes:     packageConfig.Contributes.AnycodeLanguages.Extensions,
		SuppressedBy: packageConfig.Contributes.AnycodeLanguages.SuppressedBy,
		QueryInfo:    make(map[string]string),
	}

	// 设置查询信息
	for queryType := range queries {
		langInfo.QueryInfo[queryType] = "true"
	}

	// 构建功能配置 - 根据可用的查询确定支持的功能
	featureConfig := types.FeatureConfig{
		Definitions:      hasQuery(queries, "locals") || hasQuery(queries, "outline"),
		References:       hasQuery(queries, "references") || hasQuery(queries, "identifiers"),
		Completions:      hasQuery(queries, "identifiers") || hasQuery(queries, "outline"),
		Highlights:       hasQuery(queries, "identifiers"),
		Outline:          hasQuery(queries, "outline"),
		Folding:          hasQuery(queries, "folding"),
		WorkspaceSymbols: hasQuery(queries, "outline"),
		Diagnostics:      false, // 暂时不支持诊断
	}

	return types.LanguageConfig{
		Info:    langInfo,
		Feature: featureConfig,
		Queries: queries,
	}, nil
}

// readPackageJSON 读取package.json文件
func (loader *PackageLoader) readPackageJSON(packageDir string) (*PackageConfig, error) {
	packageJSONPath := filepath.Join(packageDir, "package.json")
	
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	var config PackageConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}

	return &config, nil
}

// loadQueries 加载查询文件
func (loader *PackageLoader) loadQueries(packageDir string, queryPaths map[string]string) (map[string]string, error) {
	queries := make(map[string]string)

	for queryType, queryPath := range queryPaths {
		fullPath := filepath.Join(packageDir, queryPath)
		
		data, err := os.ReadFile(fullPath)
		if err != nil {
			// 某些查询文件可能不存在，这是正常的
			continue
		}

		queries[queryType] = string(data)
	}

	return queries, nil
}

// hasQuery 检查是否有指定的查询
func hasQuery(queries map[string]string, queryType string) bool {
	query, exists := queries[queryType]
	return exists && strings.TrimSpace(query) != ""
}

// LoadEmbeddedPackages 加载嵌入的语言包（从嵌入的文件系统）
func LoadEmbeddedPackages(fsys fs.FS) ([]types.LanguageConfig, error) {
	var configs []types.LanguageConfig

	// 扫描嵌入的文件系统
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		packageName := entry.Name()
		if !strings.HasPrefix(packageName, "anycode-") {
			continue
		}

		config, err := loadEmbeddedPackage(fsys, packageName)
		if err != nil {
			fmt.Printf("Warning: failed to load embedded package %s: %v\n", packageName, err)
			continue
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// loadEmbeddedPackage 加载嵌入的语言包
func loadEmbeddedPackage(fsys fs.FS, packageName string) (types.LanguageConfig, error) {
	// 读取package.json
	packageJSONPath := filepath.Join(packageName, "package.json")
	data, err := fs.ReadFile(fsys, packageJSONPath)
	if err != nil {
		return types.LanguageConfig{}, fmt.Errorf("failed to read package.json: %w", err)
	}

	var packageConfig PackageConfig
	if err := json.Unmarshal(data, &packageConfig); err != nil {
		return types.LanguageConfig{}, fmt.Errorf("failed to parse package.json: %w", err)
	}

	// 读取查询文件
	queries := make(map[string]string)
	for queryType, queryPath := range packageConfig.Contributes.AnycodeLanguages.QueryPaths {
		fullPath := filepath.Join(packageName, queryPath)
		
		queryData, err := fs.ReadFile(fsys, fullPath)
		if err != nil {
			// 某些查询文件可能不存在
			continue
		}

		queries[queryType] = string(queryData)
	}

	// 构建语言信息
	langInfo := types.LanguageInfo{
		ExtensionID:  packageConfig.Name,
		LanguageID:   packageConfig.Contributes.AnycodeLanguages.LanguageID,
		Suffixes:     packageConfig.Contributes.AnycodeLanguages.Extensions,
		SuppressedBy: packageConfig.Contributes.AnycodeLanguages.SuppressedBy,
		QueryInfo:    make(map[string]string),
	}

	// 设置查询信息
	for queryType := range queries {
		langInfo.QueryInfo[queryType] = "true"
	}

	// 构建功能配置
	featureConfig := types.FeatureConfig{
		Definitions:      hasQuery(queries, "locals") || hasQuery(queries, "outline"),
		References:       hasQuery(queries, "references") || hasQuery(queries, "identifiers"),
		Completions:      hasQuery(queries, "identifiers") || hasQuery(queries, "outline"),
		Highlights:       hasQuery(queries, "identifiers"),
		Outline:          hasQuery(queries, "outline"),
		Folding:          hasQuery(queries, "folding"),
		WorkspaceSymbols: hasQuery(queries, "outline"),
		Diagnostics:      false,
	}

	return types.LanguageConfig{
		Info:    langInfo,
		Feature: featureConfig,
		Queries: queries,
	}, nil
}