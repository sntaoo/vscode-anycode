package languages

import (
	"embed"
	"fmt"

	"anycode-go-server/internal/types"
)

// 嵌入语言包文件
//go:embed packages
var embeddedPackages embed.FS

// LoadBuiltinLanguages 加载内置语言包
func LoadBuiltinLanguages() ([]types.LanguageConfig, error) {
	// 尝试从嵌入的文件系统加载
	configs, err := LoadEmbeddedPackages(embeddedPackages)
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded packages: %w", err)
	}

	return configs, nil
}

// LoadLanguagesFromDirectory 从目录加载语言包
func LoadLanguagesFromDirectory(packagesDir string) ([]types.LanguageConfig, error) {
	loader := NewPackageLoader(packagesDir)
	return loader.LoadAllPackages()
}