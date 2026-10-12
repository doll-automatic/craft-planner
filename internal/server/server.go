// Package server は HTTP API と静的ファイルの配信を行う。
// 算出は internal/calc に委譲し、このパッケージでは再実装しない。
package server

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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
}

type gameListItem struct {
	GameId    string `json:"gameId"`
	Name      string `json:"name"`
	ItemCount int    `json:"itemCount"`
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
	Item      *model.Item          `json:"item"`
}

type errorBody struct {
	Error string `json:"error"`
}

// New は API と静的配信を行うサーバを組み立てる。
// dataDir はアイコン（data/icon/<gameId>/<file>）を読むディレクトリである。
func New(games []*model.GameData, errs []loader.GameLoadError, dataDir string) *Server {
	s := &Server{
		games:   map[string]*model.GameData{},
		errors:  errs,
		mux:     http.NewServeMux(),
		dataDir: dataDir,
	}
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
			GameId:    g.GameId,
			Name:      g.Name,
			ItemCount: len(g.Items),
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
		Item:       g.Items[itemID],
	})
}

func (s *Server) handleFacilities(w http.ResponseWriter, r *http.Request) {
	g, ok := s.games[r.PathValue("gameId")]
	if !ok {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "ゲームが見つかりません"})
		return
	}
	itemID := r.URL.Query().Get("itemId")
	rate, err := strconv.ParseFloat(r.URL.Query().Get("rate"), 64)
	if err != nil || rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "目標生産速度が不正です"})
		return
	}
	result, err := calc.ComputeFacilities(g, itemID, rate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
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
