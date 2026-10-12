package server

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"game-production-manager/internal/loader"
	"game-production-manager/internal/model"
)

func TestGamesAPISeparatesErrors(t *testing.T) {
	dir := t.TempDir()
	good := `
name: "良い"
items:
  - id: ore
    name: "鉱石"
    type: "中間素材"
    facility: "手"
    recipes:
      - id: ore_r
        craft_time: 1
        main_output: ore
        inputs: []
        outputs:
          - { id: ore, qty: 1 }
`
	bad := "name: [\n"
	if err := os.WriteFile(filepath.Join(dir, "good.yaml"), []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := loader.LoadGames(dir)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(res.Games, res.Errors, dir))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/games")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Games []struct {
			GameId string `json:"gameId"`
			Name   string `json:"name"`
		} `json:"games"`
		Errors []loader.GameLoadError `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Games) != 1 || body.Games[0].GameId != "good" {
		t.Fatalf("games = %+v", body.Games)
	}
	if len(body.Errors) == 0 || !strings.Contains(body.Errors[0].FileName, "bad.yaml") {
		t.Fatalf("errors = %+v", body.Errors)
	}
}

func TestTreeAndFacilitiesAPI(t *testing.T) {
	ore := &model.Item{
		ItemId: "iron_ore", Name: "鉄鉱石", Type: "中間素材", Facility: "採掘機",
		Recipes: []model.Recipe{{
			RecipeId: "iron_ore_r0", CraftTime: 2, MainOutputId: "iron_ore",
			Outputs: []model.OutputRef{{ItemId: "iron_ore", Quantity: 1}},
		}},
	}
	plate := &model.Item{
		ItemId: "iron_plate", Name: "鉄板", Type: "中間素材", Facility: "製錬炉",
		Recipes: []model.Recipe{{
			RecipeId: "iron_plate_r0", CraftTime: 3.2, MainOutputId: "iron_plate",
			Inputs:  []model.MaterialRef{{ItemId: "iron_ore", Quantity: 1}},
			Outputs: []model.OutputRef{{ItemId: "iron_plate", Quantity: 1}},
		}},
	}
	gear := &model.Item{
		ItemId: "gear", Name: "歯車", Type: "中間素材", Facility: "組立機",
		Recipes: []model.Recipe{{
			RecipeId: "gear_r0", CraftTime: 1, MainOutputId: "gear",
			Inputs:  []model.MaterialRef{{ItemId: "iron_plate", Quantity: 2}},
			Outputs: []model.OutputRef{{ItemId: "gear", Quantity: 1}},
		}},
	}
	game := &model.GameData{GameId: "sample", Name: "Sample", Items: map[string]*model.Item{
		"iron_ore": ore, "iron_plate": plate, "gear": gear,
	}}
	ts := httptest.NewServer(New([]*model.GameData{game}, nil, t.TempDir()))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/games/sample/tree?itemId=gear")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var tree struct {
		Nodes []struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"nodes"`
		Materials []struct {
			ItemId   string `json:"itemId"`
			Quantity int    `json:"quantity"`
		} `json:"materials"`
		Truncated bool `json:"truncated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		t.Fatal(err)
	}
	if len(tree.Nodes) != 3 || tree.Truncated {
		t.Fatalf("tree = %+v", tree)
	}
	qty := map[string]int{}
	for _, line := range tree.Materials {
		qty[line.ItemId] = line.Quantity
	}
	if qty["iron_plate"] != 2 || qty["iron_ore"] != 2 {
		t.Fatalf("materials = %+v", tree.Materials)
	}

	fac, err := http.Get(ts.URL + "/api/games/sample/facilities?itemId=gear&rate=2")
	if err != nil {
		t.Fatal(err)
	}
	defer fac.Body.Close()
	var result model.FacilityResult
	if err := json.NewDecoder(fac.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, it := range result.Items {
		counts[it.ItemId] = it.FacilityCount
	}
	if counts["gear"] != 2 || counts["iron_plate"] != 13 || counts["iron_ore"] != 8 {
		t.Fatalf("counts = %+v", result.Items)
	}

	for _, path := range []string{"/api/games/sample/facilities?itemId=gear&rate=0", "/api/games/sample/facilities?itemId=gear&rate=abc"} {
		bad, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(bad.Body)
		bad.Body.Close()
		if bad.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "不正") {
			t.Fatalf("%s -> %d %s", path, bad.StatusCode, body)
		}
	}
}

func TestStaticEmbed(t *testing.T) {
	ts := httptest.NewServer(New(nil, nil, t.TempDir()))
	defer ts.Close()
	index, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer index.Body.Close()
	body, _ := io.ReadAll(index.Body)
	text := string(body)
	if index.StatusCode != 200 || !strings.Contains(text, "生産管理") {
		t.Fatalf("index %d %s", index.StatusCode, text)
	}
	if strings.Contains(text, "cdn") || !strings.Contains(text, "vendor/cytoscape.min.js") {
		t.Fatalf("静的参照がローカルではない: %s", text)
	}
	lib, err := http.Get(ts.URL + "/vendor/cytoscape.min.js")
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Body.Close()
	js, _ := io.ReadAll(lib.Body)
	if lib.StatusCode != 200 || !strings.Contains(string(js), "cytoscape") {
		t.Fatalf("cytoscape status=%d len=%d", lib.StatusCode, len(js))
	}
	app, err := http.Get(ts.URL + "/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer app.Body.Close()
	if app.StatusCode != 200 {
		t.Fatal(app.StatusCode)
	}
}

func TestIconRoute(t *testing.T) {
	dir := t.TempDir()
	game := &model.GameData{
		GameId: "sample",
		Name:   "Sample",
		Items:  map[string]*model.Item{"ore": {ItemId: "ore", Name: "鉱石"}},
	}
	pngBytes := testPNG(t)
	iconPath := filepath.Join(dir, "icon", "sample", "ore.png")
	if err := os.MkdirAll(filepath.Dir(iconPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(iconPath, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("secret-body"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "icon", "other", "ore.png")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New([]*model.GameData{game}, nil, dir)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ok, err := http.Get(ts.URL + "/data/icon/sample/ore.png")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(ok.Body)
	ok.Body.Close()
	if ok.StatusCode != http.StatusOK || !strings.HasPrefix(ok.Header.Get("Content-Type"), "image/png") || !bytes.Equal(body, pngBytes) {
		t.Fatalf("icon status=%d type=%s len=%d", ok.StatusCode, ok.Header.Get("Content-Type"), len(body))
	}

	for _, path := range []string{
		"/data/icon/sample/missing.png",
		"/data/icon/other/ore.png",
		"/data/icon/nope/ore.png",
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound || bytes.Contains(got, []byte("secret-body")) {
			t.Fatalf("%s -> %d %q", path, resp.StatusCode, got)
		}
	}

	for _, file := range []string{"../secret.txt", `..\..\secret.txt`, "..", "ore.png/../../secret.txt"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/data/icon/sample/ore.png", nil)
		req.SetPathValue("gameId", "sample")
		req.SetPathValue("file", file)
		srv.handleIcon(rec, req)
		if rec.Code != http.StatusNotFound || bytes.Contains(rec.Body.Bytes(), []byte("secret-body")) {
			t.Fatalf("file %q -> %d %q", file, rec.Code, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/data/icon/sample/ore.png", nil)
	req.SetPathValue("gameId", "..")
	req.SetPathValue("file", "secret.txt")
	srv.handleIcon(rec, req)
	if rec.Code != http.StatusNotFound || bytes.Contains(rec.Body.Bytes(), []byte("secret-body")) {
		t.Fatalf("gameId .. -> %d %q", rec.Code, rec.Body.String())
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 12, G: 180, B: 90, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
