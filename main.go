// 生産管理ツールのエントリポイント。
// ポート検証のあと YAML を読み、Listen に成功してから、このPC用と LAN 用のアクセス URL を 1 回ずつ出力する。
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"game-production-manager/internal/loader"
	"game-production-manager/internal/server"
)

// Runtime は Listen 済みのサーバである。Serve は呼び出し側が行う。
type Runtime struct {
	Listener net.Listener
	Handler  http.Handler
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

// Boot はポートを検証し、データを読み、Listen 後にアクセス URL を 1 回ずつ出力する。
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
		result = loader.LoadResult{Errors: []loader.GameLoadError{{
			Message: err.Error(),
		}}}
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Fprintf(stdout, "ポートが使用中です: %d (%v)\n", port, err)
		return nil, 1
	}
	urls := accessURLs(port)
	for _, url := range urls {
		fmt.Fprintln(stdout, url)
	}
	return &Runtime{
		Listener: ln,
		Handler:  server.New(result.Games, result.Errors, dataDir, clientJoinURL(urls)),
	}, 0
}

// clientJoinURL は QR に出すクライアント画面の URL である。
// 起動ログと同じ順で、最初の LAN アドレスを使う。無ければ localhost。
func clientJoinURL(urls []string) string {
	base := urls[0]
	for _, u := range urls {
		if !strings.Contains(u, "://localhost") {
			base = u
			break
		}
	}
	return strings.TrimRight(base, "/") + "/client"
}

// accessURLs は自分のブラウザ用と、外部へ通信できる NIC の URL を返す。
// 169.254.0.0/16 のようなリンクローカルは、同じ LAN のタブレットから開けない。
func accessURLs(port int) []string {
	urls := []string{fmt.Sprintf("http://localhost:%d/", port)}
	external := externalInterfaceIndexes()
	if len(external) == 0 {
		return urls
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return urls
	}
	seen := map[string]bool{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || !external[iface.Index] {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := ipv4Of(addr)
			if !routableIPv4(ip) || seen[ip.String()] {
				continue
			}
			seen[ip.String()] = true
			urls = append(urls, fmt.Sprintf("http://%s:%d/", ip, port))
		}
	}
	return urls
}

func ipv4Of(addr net.Addr) net.IP {
	switch a := addr.(type) {
	case *net.IPNet:
		return a.IP.To4()
	case *net.IPAddr:
		return a.IP.To4()
	default:
		return nil
	}
}

// routableIPv4 は他の端末から到達し得る IPv4 か。リンクローカルは対象外。
func routableIPv4(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast() && !ip.IsUnspecified()
}

// ValidatePort は 1024 以上 65535 以下の整数だけを受け付ける。
func ValidatePort(input string) (int, error) {
	port, err := strconv.Atoi(input)
	if err != nil || port < 1024 || port > 65535 {
		return 0, fmt.Errorf("指定ポートが不正です: %s", input)
	}
	return port, nil
}
