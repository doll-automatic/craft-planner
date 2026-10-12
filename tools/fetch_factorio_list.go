package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type TreeResponse struct {
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
}

func main() {
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/wube/factorio-data/git/trees/master?recursive=1", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var tr TreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		fmt.Printf("Decode error: %v\n", err)
		return
	}

	iconMap := make(map[string]string) // filename -> full path
	for _, item := range tr.Tree {
		if strings.HasSuffix(item.Path, ".png") && strings.Contains(item.Path, "icons") {
			parts := strings.Split(item.Path, "/")
			filename := parts[len(parts)-1]
			iconMap[filename] = item.Path
		}
	}
	fmt.Printf("Found %d icons in factorio-data\n", len(iconMap))

	b, _ := json.MarshalIndent(iconMap, "", "  ")
	os.WriteFile("tools/factorio_icons.json", b, 0644)
}
