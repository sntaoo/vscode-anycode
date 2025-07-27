# AnyCode Go Server

这是一个用Go语言重写的anycode语言服务器，基于tree-sitter提供多语言的LSP功能。

## 项目结构

```
go-server/
├── main.go                          # 主入口文件
├── go.mod                           # Go模块文件
├── internal/
│   ├── types/                       # 核心数据类型定义
│   │   └── types.go
│   ├── storage/                     # 符号存储实现
│   │   └── storage.go
│   ├── documents/                   # 文档管理
│   │   └── store.go
│   ├── languages/                   # 语言管理器
│   │   └── manager.go
│   ├── trees/                       # Tree-sitter解析器管理
│   │   └── parser.go
│   ├── symbols/                     # 符号索引
│   │   └── index.go
│   ├── providers/                   # LSP功能提供者
│   │   ├── definitions.go          # 定义跳转
│   │   ├── completion.go           # 代码补全
│   │   └── references.go           # 引用查找
│   └── server/                      # 主服务器实现
│       └── server.go
└── README.md
```

## 核心组件

### 1. Tree-sitter解析器管理 (`trees/parser.go`)
- 管理tree-sitter解析器实例
- 实现解析树缓存和增量解析
- 监听文档变更事件并更新解析树

### 2. 语言管理器 (`languages/manager.go`)
- 管理多种编程语言的语法和查询
- 支持动态加载语言数据
- 根据文件扩展名识别语言类型

### 3. 符号索引 (`symbols/index.go`)
- 构建和维护全局符号索引
- 支持符号定义和引用的快速查找
- 实现增量索引更新

### 4. 功能提供者 (`providers/`)
- **定义跳转**: 支持局部和全局定义查找
- **代码补全**: 提供本地符号、全局符号和关键字补全
- **引用查找**: 查找符号的所有引用位置

### 5. 存储层 (`storage/storage.go`)
- 提供符号信息存储接口
- 实现内存存储，可扩展为持久化存储

### 6. 语言包系统 (`languages/package.go`)
- 自动加载anycode语言包
- 支持内置和外部语言包
- 解析package.json配置和查询文件
- 动态功能配置

## 主要特性

1. **多语言支持**: 通过tree-sitter支持多种编程语言
2. **语言包集成**: 自动加载anycode语言包和查询规则
3. **增量解析**: 高效的增量解析和缓存机制
4. **符号索引**: 快速的全局符号索引和查找
5. **LSP兼容**: 完全兼容Language Server Protocol
6. **模块化设计**: 清晰的组件分离和接口设计
7. **高性能**: 基于Go语言的高并发处理能力
8. **嵌入式语言包**: 内置常用语言包，开箱即用

## 使用方法

### 编译和运行

```bash
# 进入项目目录
cd go-server

# 下载依赖
go mod tidy

# 编译
go build -o anycode-server

# 运行 (stdio模式，使用内置语言包)
./anycode-server -mode=stdio

# 运行 (stdio模式，同时加载外部语言包)
./anycode-server -mode=stdio -packages=../

# 运行 (TCP模式)
./anycode-server -mode=tcp -addr=:4389

# 使用Makefile构建和运行
make run-stdio
make run-stdio-with-packages
make run-tcp
```

### 集成到编辑器

#### VS Code配置示例

```json
{
  "anycode.enableGoServer": true,
  "anycode.goServerPath": "/path/to/anycode-server",
  "anycode.goServerArgs": ["-mode=stdio"]
}
```

## 核心接口

### Provider接口
```go
type Provider interface {
    Register(server LSPServer) error
}
```

### LSPServer接口
```go
type LSPServer interface {
    RegisterHandler(method string, handler interface{}) error
    RegisterCapability(method string, selector []string) error
}
```

### 存储接口
```go
type SymbolInfoStorage interface {
    Insert(uri string, info map[string]*SymbolInfo) error
    GetAll() (map[string]map[string]*SymbolInfo, error)
    Delete(uris []string) error
}
```

## 语言包系统

### 内置语言包

服务器内置了以下语言包：
- **Java**: 完整的类、方法、字段、接口支持
- **Go**: 函数、结构体、接口、变量支持  
- **Python**: 类、函数、变量支持

### 语言包结构

每个语言包包含：
```
anycode-{language}/
├── package.json          # 语言包配置
└── queries/              # Tree-sitter查询文件
    ├── comments.scm      # 注释查询
    ├── identifiers.scm   # 标识符查询
    ├── locals.scm        # 局部变量查询
    ├── outline.scm       # 大纲查询
    ├── references.scm    # 引用查询
    └── folding.scm       # 代码折叠查询
```

### 查询文件说明

- **outline.scm**: 定义类、函数、变量等符号的提取规则
- **locals.scm**: 定义作用域和局部变量的查询规则
- **identifiers.scm**: 定义标识符的匹配规则
- **references.scm**: 定义引用查找的规则
- **comments.scm**: 定义注释的匹配规则
- **folding.scm**: 定义代码折叠的规则

## 扩展开发

### 添加新的语言支持

#### 方法1: 创建语言包
1. 在`internal/languages/packages/`下创建`anycode-{language}`目录
2. 创建`package.json`配置文件
3. 在`queries/`目录下添加相应的`.scm`查询文件
4. 重新编译服务器

#### 方法2: 外部语言包目录
1. 创建语言包目录结构
2. 使用`-packages`参数指定目录
3. 服务器会自动扫描并加载语言包

### 添加新的LSP功能

1. 实现Provider接口
2. 在server中注册新的provider
3. 添加对应的LSP方法处理

### 自定义存储后端

1. 实现SymbolInfoStorage接口
2. 创建对应的StorageFactory
3. 在main函数中切换存储实现

## 依赖项

- `github.com/smacker/go-tree-sitter`: Tree-sitter Go绑定
- `github.com/sourcegraph/go-lsp`: LSP协议实现
- `github.com/sourcegraph/jsonrpc2`: JSON-RPC 2.0实现

## 性能优化

1. **解析缓存**: 实现LRU缓存避免重复解析
2. **增量更新**: 文档变更时只更新必要的部分
3. **并发处理**: 利用Go的goroutine实现并发索引
4. **内存管理**: 及时释放不再使用的tree-sitter对象

## 贡献指南

1. Fork此项目
2. 创建功能分支
3. 提交变更
4. 创建Pull Request

## 许可证

MIT License