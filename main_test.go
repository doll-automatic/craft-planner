package main

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestValidatePort(t *testing.T) {
	for _, input := range []string{"1024", "8080", "65535"} {
		if _, err := ValidatePort(input); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
	}
	for _, input := range []string{"80", "1023", "65536", "abc", "1.5", "-1", ""} {
		if _, err := ValidatePort(input); err == nil {
			t.Fatalf("%s を受け付けた", input)
		}
	}
}

func TestInvalidPortDoesNotStart(t *testing.T) {
	for _, input := range []string{"80", "abc", "70000"} {
		var buf strings.Builder
		rt, code := Boot([]string{input, t.TempDir()}, &buf)
		if code == 0 || rt != nil {
			if rt != nil {
				rt.Listener.Close()
			}
			t.Fatalf("%s で起動した", input)
		}
		msg := buf.String()
		if !strings.Contains(msg, "不正") || !strings.Contains(msg, input) {
			t.Fatalf("%s: %s", input, msg)
		}
		if strings.Contains(msg, "http://") {
			t.Fatalf("失敗時に URL が出た: %s", msg)
		}
	}
}

func TestStartupURLAndBusyPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	var buf strings.Builder
	start := time.Now()
	rt, code := Boot([]string{strconv.Itoa(port), t.TempDir()}, &buf)
	elapsed := time.Since(start)
	if code != 0 {
		t.Fatal(buf.String())
	}
	if elapsed > 5*time.Second {
		t.Fatalf("起動に %s かかった", elapsed)
	}
	wantURL := "http://localhost:" + strconv.Itoa(port) + "/"
	if strings.Count(buf.String(), wantURL) != 1 {
		t.Fatalf("URL出力 = %q", buf.String())
	}

	srv := &http.Server{Handler: rt.Handler}
	go srv.Serve(rt.Listener)
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "vendor/cytoscape.min.js") {
		t.Fatalf("配信失敗 %d", resp.StatusCode)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("受付可能まで %s", time.Since(start))
	}

	var busy strings.Builder
	rt2, code2 := Boot([]string{strconv.Itoa(port), t.TempDir()}, &busy)
	if code2 == 0 {
		if rt2 != nil {
			rt2.Listener.Close()
		}
		t.Fatal("使用中ポートで起動した")
	}
	msg := busy.String()
	if !strings.Contains(msg, "使用中") || !strings.Contains(msg, strconv.Itoa(port)) {
		t.Fatalf("使用中メッセージ = %q", msg)
	}
	if strings.Contains(msg, "http://") {
		t.Fatalf("失敗時に URL が出た: %s", msg)
	}
}
