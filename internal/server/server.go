// Package server は HTTP API と静的ファイルの配信を行う。
// 算出は internal/calc に委譲し、このパッケージでは再実装しない。
package server

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"game-production-manager/internal/calc"
	"game-production-manager/internal/loader"
	"game-production-manager/internal/model"
	"game-production-manager/web"
)

const maxTreeNodes = 1000

// Server は検証済みゲームと HTTP ハンドラを保持する。
type Server struct {
	games   map[string]*model.GameData
	order   []*model.GameData
	errors  []loader.GameLoadError
	mux     *http.ServeMux
	dataDir string
	joinURL string
	mu      sync.Mutex
	state   sharedState
}

// sharedState はホストとクライアントが共有する選択である。
// 種類の絞り込み、表示切替、テーマ、全画面は含めない。
type sharedState struct {
	GameId   string `json:"gameId"`
	ItemId   string `json:"itemId"`
	Quantity string `json:"quantity"`
	Revision int64  `json:"revision"`
}

// statePatch は送られた項目だけを更新する。nil は未指定。
type statePatch struct {
	GameId   *string `json:"gameId"`
	ItemId   *string `json:"itemId"`
	Quantity *string `json:"quantity"`
}

type gameListItem struct {
	GameId string `json:"gameId"`
	Name   string `json:"name"`
}

type gamesResponse struct {
	Games  []gameListItem         `json:"games"`
	Errors []loader.GameLoadError `json:"errors"`
}

type gameDetail struct {
	GameId string        `json:"gameId"`
	Name   string        `json:"name"`
	Items  []*model.Item `json:"items"`
}

type treeResponse struct {
	model.TreeResult
	Materials []model.MaterialLine `json:"materials"`
}

type errorBody struct {
	Error string `json:"error"`
}

// planResponse は目標個数に対する施設数と、同じ個数ぶんの素材合計である。
type planResponse struct {
	Items     []model.FacilityItem `json:"items"`
	Materials []model.MaterialLine `json:"materials"`
}

// New は API と静的配信を行うサーバを組み立てる。
// dataDir はアイコン（data/icon/<gameId>/<file>）を読むディレクトリである。
// joinURL はクライアント画面の接続先で、QR コードに出す。
func New(games []*model.GameData, errs []loader.GameLoadError, dataDir, joinURL string) *Server {
	s := &Server{
		games:   map[string]*model.GameData{},
		errors:  errs,
		mux:     http.NewServeMux(),
		dataDir: dataDir,
		joinURL: joinURL,
	}
	// 変更番号は起動時刻（ミリ秒）から数える。再起動しても前回の番号より大きくなり、
	// 開いたままの画面が「古い番号」と見なして変更を無視することがない。
	s.state.Revision = time.Now().UnixMilli()
	if s.errors == nil {
		s.errors = []loader.GameLoadError{}
	}
	for _, g := range games {
		if g == nil {
			continue
		}
		if _, exists := s.games[g.GameId]; exists {
			continue
		}
		s.games[g.GameId] = g
		s.order = append(s.order, g)
	}
	sort.Slice(s.order, func(i, j int) bool { return s.order[i].GameId < s.order[j].GameId })

	s.mux.HandleFunc("GET /api/games", s.handleGames)
	s.mux.HandleFunc("GET /api/games/{gameId}/tree", s.handleTree)
	s.mux.HandleFunc("GET /api/games/{gameId}/facilities", s.handleFacilities)
	s.mux.HandleFunc("GET /api/games/{gameId}", s.handleGame)
	s.mux.HandleFunc("GET /api/state", s.handleGetState)
	s.mux.HandleFunc("PATCH /api/state", s.handlePatchState)
	s.mux.HandleFunc("GET /api/join", s.handleJoin)
	s.mux.HandleFunc("GET /client", s.handleClient)
	s.mux.HandleFunc("GET /data/icon/{gameId}/{file}", s.handleIcon)
	s.mux.Handle("/", http.FileServer(http.FS(web.FS)))
	return s
}

// ServeHTTP は HTTP リクエストを振り分ける。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	list := make([]gameListItem, 0, len(s.order))
	for _, g := range s.order {
		list = append(list, gameListItem{
			GameId: g.GameId,
			Name:   g.Name,
		})
	}
	writeJSON(w, http.StatusOK, gamesResponse{Games: list, Errors: s.errors})
}

func (s *Server) handleGame(w http.ResponseWriter, r *http.Request) {
	g, ok := s.games[r.PathValue("gameId")]
	if !ok {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "ゲームが見つかりません"})
		return
	}
	items := make([]*model.Item, 0, len(g.Items))
	for _, it := range g.Items {
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ItemId < items[j].ItemId })
	writeJSON(w, http.StatusOK, gameDetail{GameId: g.GameId, Name: g.Name, Items: items})
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	g, ok := s.games[r.PathValue("gameId")]
	if !ok {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "ゲームが見つかりません"})
		return
	}
	itemID := r.URL.Query().Get("itemId")
	tree, err := calc.BuildTree(g, itemID, maxTreeNodes)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorBody{Error: err.Error()})
		return
	}
	materials, err := calc.SummarizeMaterials(g, itemID, maxTreeNodes)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorBody{Error: err.Error()})
		return
	}
	if materials == nil {
		materials = []model.MaterialLine{}
	}
	writeJSON(w, http.StatusOK, treeResponse{
		TreeResult: tree,
		Materials:  materials,
	})
}

func (s *Server) handleFacilities(w http.ResponseWriter, r *http.Request) {
	g, ok := s.games[r.PathValue("gameId")]
	if !ok {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "ゲームが見つかりません"})
		return
	}
	itemID := r.URL.Query().Get("itemId")
	quantity, err := parseTargetQuantity(r.URL.Query().Get("quantity"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "目標個数が不正です"})
		return
	}
	result, err := calc.ComputeFacilities(g, itemID, quantity)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	lines, err := calc.SummarizeMaterials(g, itemID, maxTreeNodes)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	materials, err := calc.ScaleMaterials(lines, quantity)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	if result.Items == nil {
		result.Items = []model.FacilityItem{}
	}
	writeJSON(w, http.StatusOK, planResponse{Items: result.Items, Materials: materials})
}

func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	state := s.state
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handlePatchState(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var patch statePatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "状態が不正です"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.state
	if patch.GameId != nil {
		if *patch.GameId != next.GameId && patch.ItemId == nil {
			next.ItemId = ""
		}
		next.GameId = *patch.GameId
	}
	if patch.ItemId != nil {
		next.ItemId = *patch.ItemId
	}
	if patch.Quantity != nil {
		next.Quantity = strings.TrimSpace(*patch.Quantity)
	}
	if err := s.validateState(next); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	if next.GameId != s.state.GameId || next.ItemId != s.state.ItemId || next.Quantity != s.state.Quantity {
		next.Revision = s.state.Revision + 1
		s.state = next
	}
	writeJSON(w, http.StatusOK, s.state)
}

func (s *Server) validateState(state sharedState) error {
	if state.GameId != "" {
		if _, ok := s.games[state.GameId]; !ok {
			return fmt.Errorf("ゲームが見つかりません")
		}
	}
	if state.ItemId != "" {
		game := s.games[state.GameId]
		if game == nil || game.Items[state.ItemId] == nil {
			return fmt.Errorf("アイテムが見つかりません")
		}
	}
	if state.Quantity != "" {
		if _, err := parseTargetQuantity(state.Quantity); err != nil {
			return fmt.Errorf("目標速度が不正です")
		}
	}
	return nil
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		URL string `json:"url"`
	}{URL: s.joinURL})
}

func (s *Server) handleClient(w http.ResponseWriter, r *http.Request) {
	data, err := web.FS.ReadFile("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// parseTargetQuantity は 0 より大きい目標速度（個/秒）だけを受け付ける。
func parseTargetQuantity(raw string) (float64, error) {
	if raw == "" || strings.HasPrefix(raw, "+") {
		return 0, fmt.Errorf("目標速度が不正です")
	}
	q, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(q) || math.IsInf(q, 0) || q <= 0 || q > 1_000_000_000 {
		return 0, fmt.Errorf("目標速度が不正です")
	}
	return q, nil
}

// handleIcon は data/icon/<gameId>/<file> を配信する。無いファイルは 404。
// gameId は読込済みゲームだけを許し、パス区切りと ".." によるディレクトリ外参照は拒否する。
func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameId")
	name := r.PathValue("file")
	if !safeIconSegment(gameID) || !safeIconSegment(name) {
		http.NotFound(w, r)
		return
	}
	if _, ok := s.games[gameID]; !ok {
		http.NotFound(w, r)
		return
	}
	root := filepath.Join(s.dataDir, "icon", gameID)
	target := filepath.Join(root, name)
	if !pathWithin(root, target) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Lstat(target)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil || !pathWithin(resolvedRoot, resolved) {
		http.NotFound(w, r)
		return
	}
	resolvedInfo, err := os.Stat(resolved)
	if err != nil || !resolvedInfo.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, resolved)
}

// safeIconSegment はパス要素 1 つとして安全な名前だけを通す。
func safeIconSegment(name string) bool {
	if name == "" || name == "." || name == ".." || strings.Contains(name, "..") {
		return false
	}
	if strings.ContainsRune(name, 0) {
		return false
	}
	return !strings.ContainsAny(name, `/\`)
}

// pathWithin は target が root の直下より内側にあるときだけ true を返す。
func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil || rel == "." {
		return false
	}
	return filepath.IsLocal(rel)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
