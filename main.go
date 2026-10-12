// Game_Production_Manager のエントリポイント。
// ポート検証のあと YAML を読み、Listen に成功してからアクセス URL を 1 回だけ出力する。
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"

	"game-production-manager/internal/loader"
	"game-production-manager/internal/server"
)

// Runtime は Listen 済みのサーバである。Serve は呼び出し側が行う。
type Runtime struct {
	Listener net.Listener
	Handler  http.Handler
	Port     int
}

func main() {
	rt, code := Boot(os.Args[1:], os.Stdout)
	if code != 0 {
		os.Exit(code)
	}
	if err := http.Serve(rt.Listener, rt.Handler); err != nil {
		fmt.Fprintln(os.Stdout, err)
		os.Exit(1)
	}
}

// Boot はポートを検証し、データを読み、Listen 後に URL を 1 回出力する。
// ポートが不正、または bind に失敗した場合は URL を出さず終了コード 1 を返す。
func Boot(args []string, stdout io.Writer) (*Runtime, int) {
	portStr := "8080"
	dataDir := "data"
	if len(args) >= 1 && args[0] != "" {
		portStr = args[0]
	}
	if len(args) >= 2 && args[1] != "" {
		dataDir = args[1]
	}

	port, err := ValidatePort(portStr)
	if err != nil {
		fmt.Fprintf(stdout, "指定ポートが不正です: %s\n", portStr)
		return nil, 1
	}

	result, err := loader.LoadGames(dataDir)
	if err != nil {
		fmt.Fprintf(stdout, "データディレクトリの読み込みに失敗しました: %v\n", err)
		result = loader.LoadResult{Games: nil, Errors: nil}
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Fprintf(stdout, "ポートが使用中です: %d\n", port)
		return nil, 1
	}
	fmt.Fprintf(stdout, "http://localhost:%d/\n", port)
	return &Runtime{
		Listener: ln,
		Handler:  server.New(result.Games, result.Errors, dataDir),
		Port:     port,
	}, 0
}

// ValidatePort は 1024 以上 65535 以下の整数だけを受け付ける。
func ValidatePort(input string) (int, error) {
	port, err := strconv.Atoi(input)
	if err != nil || port < 1024 || port > 65535 {
		return 0, fmt.Errorf("指定ポートが不正です: %s", input)
	}
	return port, nil
}
