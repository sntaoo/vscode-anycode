package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"anycode-go-server/internal/server"
	"anycode-go-server/internal/storage"

	"github.com/sourcegraph/jsonrpc2"
)

func main() {
	var (
		mode        = flag.String("mode", "stdio", "communication mode (stdio, tcp)")
		addr        = flag.String("addr", ":4389", "server address (for tcp mode)")
		packagesDir = flag.String("packages", "", "directory containing language packages (optional)")
	)
	flag.Parse()

	// 创建存储工厂
	factory := &storage.MemoryStorageFactory{}

	// 创建服务器
	srv := server.NewServer(factory)
	
	// 如果指定了packages目录，从目录加载语言包
	if *packagesDir != "" {
		if err := srv.LoadLanguagesFromDirectory(*packagesDir); err != nil {
			log.Printf("Warning: failed to load languages from directory %s: %v", *packagesDir, err)
		}
	}

	var conn *jsonrpc2.Conn
	var err error

	switch *mode {
	case "stdio":
		conn = jsonrpc2.NewConn(
			context.Background(),
			jsonrpc2.NewBufferedStream(os.Stdin, os.Stdout, jsonrpc2.VSCodeObjectCodec{}),
			srv,
		)
	case "tcp":
		listener, err := net.Listen("tcp", *addr)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Language server listening on %s\n", *addr)
		
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Error accepting connection: %v", err)
				continue
			}
			
			go func() {
				jsonrpc2.NewConn(
					context.Background(),
					jsonrpc2.NewBufferedStream(conn, conn, jsonrpc2.VSCodeObjectCodec{}),
					srv,
				)
			}()
		}
	default:
		log.Fatalf("Unknown mode: %s", *mode)
	}

	if err != nil {
		log.Fatal(err)
	}

	// 等待连接关闭
	<-conn.DisconnectNotify()
}