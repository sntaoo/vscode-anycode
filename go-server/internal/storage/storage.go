package storage

import (
	"sync"

	"anycode-go-server/internal/types"
)

// SymbolInfoStorage 符号信息存储接口
type SymbolInfoStorage interface {
	Insert(uri string, info map[string]*types.SymbolInfo) error
	GetAll() (map[string]map[string]*types.SymbolInfo, error)
	Delete(uris []string) error
}

// StorageFactory 存储工厂接口
type StorageFactory interface {
	Create(name string) (SymbolInfoStorage, error)
	Destroy(storage SymbolInfoStorage) error
}

// MemorySymbolStorage 内存符号存储
type MemorySymbolStorage struct {
	mu   sync.RWMutex
	data map[string]map[string]*types.SymbolInfo
}

// NewMemorySymbolStorage 创建内存符号存储
func NewMemorySymbolStorage() *MemorySymbolStorage {
	return &MemorySymbolStorage{
		data: make(map[string]map[string]*types.SymbolInfo),
	}
}

// Insert 插入符号信息
func (s *MemorySymbolStorage) Insert(uri string, info map[string]*types.SymbolInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[uri] = info
	return nil
}

// GetAll 获取所有符号信息
func (s *MemorySymbolStorage) GetAll() (map[string]map[string]*types.SymbolInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make(map[string]map[string]*types.SymbolInfo)
	for uri, symbols := range s.data {
		symbolsCopy := make(map[string]*types.SymbolInfo)
		for name, info := range symbols {
			symbolsCopy[name] = info
		}
		result[uri] = symbolsCopy
	}
	return result, nil
}

// Delete 删除符号信息
func (s *MemorySymbolStorage) Delete(uris []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for _, uri := range uris {
		delete(s.data, uri)
	}
	return nil
}

// MemoryStorageFactory 内存存储工厂
type MemoryStorageFactory struct{}

// Create 创建存储实例
func (f *MemoryStorageFactory) Create(name string) (SymbolInfoStorage, error) {
	return NewMemorySymbolStorage(), nil
}

// Destroy 销毁存储实例
func (f *MemoryStorageFactory) Destroy(storage SymbolInfoStorage) error {
	// 内存存储无需特殊清理
	return nil
}