package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type WikiResponse struct {
	Query struct {
		Pages map[string]struct {
			ImageInfo []struct {
				URL string `json:"url"`
			} `json:"imageinfo"`
		} `json:"pages"`
	} `json:"query"`
}

func main() {
	endpoint := "https://satisfactory.wiki.gg/api.php?action=query&titles=File:Iron_Ingot.png&prop=imageinfo&iiprop=url&format=json"
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Err: %v\n", err)
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("Raw response: %s\n", string(b))
	var wr WikiResponse
	json.Unmarshal(b, &wr)
	for _, p := range wr.Query.Pages {
		for _, ii := range p.ImageInfo {
			fmt.Printf("Iron Ingot URL: %s\n", ii.URL)
		}
	}
}
