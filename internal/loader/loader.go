// Package loader はゲーム YAML の読込と検証を行う。
// 外部データは信頼せず、検証を通過した GameData だけを算出層へ渡す。
package loader

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"game-production-manager/internal/model"
	"gopkg.in/yaml.v3"
)

const (
	minNameRunes = 1
	maxNameRunes = 100
	minRecipes   = 1
	maxRecipes   = 20
	maxInputs    = 50
	minQty       = 1
	maxQty       = 999999
	minCraftTime = 0.1
	maxCraftTime = 86400
)

// GameLoadError は 1 件の検証失敗である。失敗したゲームはファイル単位で分離する。
type GameLoadError struct {
	FileName  string   `json:"fileName"`
	ItemId    string   `json:"itemId,omitempty"`
	RefItemId string   `json:"refItemId,omitempty"`
	Cycle     []string `json:"cycle,omitempty"`
	Message   string   `json:"message"`
}

// LoadResult は全ゲーム読込の結果である。
type LoadResult struct {
	Games  []*model.GameData `json:"games"`
	Errors []GameLoadError   `json:"errors"`
}

type rawGame struct {
	Name  *string     `yaml:"name"`
	Items []yaml.Node `yaml:"items"`
}

type rawItem struct {
	ID       *string     `yaml:"id"`
	Name     *string     `yaml:"name"`
	Type     *string     `yaml:"type"`
	Facility *string     `yaml:"facility"`
	Recipes  []yaml.Node `yaml:"recipes"`
}

type rawRecipe struct {
	ID         *string    `yaml:"id"`
	CraftTime  yamlScalar `yaml:"craft_time"`
	Inputs     []rawRef   `yaml:"inputs"`
	Outputs    []rawRef   `yaml:"outputs"`
	MainOutput *string    `yaml:"main_output"`
}

type rawRef struct {
	ID  *string    `yaml:"id"`
	Qty yamlScalar `yaml:"qty"`
}

// yamlScalar は数値フィールドの元ノードを保持し、欠損と型違いを区別する。
// yaml.v3 はスカラーを yaml.Node へ直接デコードできないため、独自に受け取る。
type yamlScalar struct {
	node yaml.Node
	ok   bool
}

func (s *yamlScalar) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	s.node = *value
	s.ok = true
	return nil
}

func (s *yamlScalar) ptr() *yaml.Node {
	if s == nil || !s.ok {
		return nil
	}
	return &s.node
}

type parsedItem struct {
	item model.Item
}

// LoadGames はデータディレクトリ内の .yaml / .yml をすべて読み込み、検証する。
// 1 ファイルの失敗は他ファイルの読込を止めない。ディレクトリ自体が読めないときだけ error を返す。
func LoadGames(dataDir string) (LoadResult, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return LoadResult{}, fmt.Errorf("データディレクトリを読み込めません: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		if !isGameFile(ent.Name()) {
			continue
		}
		names = append(names, ent.Name())
	}
	sort.Strings(names)

	var result LoadResult
	seenGame := map[string]struct{}{}
	for _, name := range names {
		path := filepath.Join(dataDir, name)
		game, errs := loadFile(path, name)
		if len(errs) > 0 {
			result.Errors = append(result.Errors, errs...)
			continue
		}
		if _, ok := seenGame[game.GameId]; ok {
			result.Errors = append(result.Errors, GameLoadError{
				FileName: name,
				Message:  "ゲームIDが他のファイルと重複しています: " + game.GameId,
			})
			continue
		}
		seenGame[game.GameId] = struct{}{}
		result.Games = append(result.Games, game)
	}
	if result.Games == nil {
		result.Games = []*model.GameData{}
	}
	if result.Errors == nil {
		result.Errors = []GameLoadError{}
	}
	return result, nil
}

func isGameFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func loadFile(path, fileName string) (*model.GameData, []GameLoadError) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []GameLoadError{{
			FileName: fileName,
			Message:  "ファイルを読み込めません: " + err.Error(),
		}}
	}

	var doc rawGame
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, []GameLoadError{{
			FileName: fileName,
			Message:  "YAMLの構文が不正です: " + err.Error(),
		}}
	}

	var errs []GameLoadError
	if doc.Name == nil || strings.TrimSpace(*doc.Name) == "" {
		errs = append(errs, GameLoadError{
			FileName: fileName,
			Message:  "ゲーム名が欠損しています",
		})
	}

	items := make([]parsedItem, 0, len(doc.Items))
	for _, node := range doc.Items {
		item, itemErrs := parseItem(fileName, node)
		errs = append(errs, itemErrs...)
		items = append(items, item)
	}

	errs = append(errs, duplicateItemErrors(fileName, items)...)
	errs = append(errs, duplicateRecipeErrors(fileName, items)...)
	idSet := itemIDSet(items)
	errs = append(errs, referenceErrors(fileName, items, idSet)...)
	errs = append(errs, craftTimeErrors(fileName, doc.Items)...)
	errs = append(errs, quantityErrors(fileName, doc.Items)...)
	errs = append(errs, lengthAndCountErrors(fileName, items)...)

	byID := itemsByID(items)
	if cycle := detectCycle(byID); len(cycle) > 0 {
		itemID := ""
		if len(cycle) > 0 {
			itemID = cycle[0]
		}
		errs = append(errs, GameLoadError{
			FileName: fileName,
			ItemId:   itemID,
			Cycle:    cycle,
			Message:  "循環参照が存在します: " + strings.Join(cycle, " → "),
		})
	}

	if len(errs) > 0 {
		return nil, errs
	}

	name := ""
	if doc.Name != nil {
		name = *doc.Name
	}
	game := &model.GameData{
		GameId: gameIDFromFile(fileName),
		Name:   name,
		Items:  byID,
	}
	return game, nil
}

func gameIDFromFile(fileName string) string {
	return strings.TrimSuffix(fileName, filepath.Ext(fileName))
}

func parseItem(fileName string, node yaml.Node) (parsedItem, []GameLoadError) {
	var raw rawItem
	if err := node.Decode(&raw); err != nil {
		id, _ := mappingScalar(&node, "id")
		return parsedItem{}, []GameLoadError{{
			FileName: fileName,
			ItemId:   id,
			Message:  "アイテム定義を解釈できません: " + err.Error(),
		}}
	}

	item := model.Item{
		ItemId:   deref(raw.ID),
		Name:     deref(raw.Name),
		Type:     deref(raw.Type),
		Facility: deref(raw.Facility),
	}
	var errs []GameLoadError
	if raw.ID == nil || item.ItemId == "" {
		errs = append(errs, GameLoadError{FileName: fileName, Message: "アイテムIDが欠損しています"})
	}
	if raw.Name == nil || item.Name == "" {
		errs = append(errs, GameLoadError{FileName: fileName, ItemId: item.ItemId, Message: "アイテム名が欠損しています"})
	}
	if raw.Type == nil || item.Type == "" {
		errs = append(errs, GameLoadError{FileName: fileName, ItemId: item.ItemId, Message: "種別が欠損しています"})
	}
	if raw.Facility == nil || item.Facility == "" {
		errs = append(errs, GameLoadError{FileName: fileName, ItemId: item.ItemId, Message: "生産施設が欠損しています"})
	}

	for _, recipeNode := range raw.Recipes {
		recipe, recipeErrs := parseRecipe(fileName, item.ItemId, recipeNode)
		errs = append(errs, recipeErrs...)
		item.Recipes = append(item.Recipes, recipe)
	}
	return parsedItem{item: item}, errs
}

func parseRecipe(fileName, itemID string, node yaml.Node) (model.Recipe, []GameLoadError) {
	var raw rawRecipe
	if err := node.Decode(&raw); err != nil {
		return model.Recipe{}, []GameLoadError{{
			FileName: fileName,
			ItemId:   itemID,
			Message:  "レシピ定義を解釈できません: " + err.Error(),
		}}
	}

	recipe := model.Recipe{
		RecipeId:     deref(raw.ID),
		MainOutputId: deref(raw.MainOutput),
	}
	var errs []GameLoadError
	if raw.ID == nil || recipe.RecipeId == "" {
		errs = append(errs, GameLoadError{FileName: fileName, ItemId: itemID, Message: "レシピIDが欠損しています"})
	}
	if raw.MainOutput == nil || recipe.MainOutputId == "" {
		errs = append(errs, GameLoadError{FileName: fileName, ItemId: itemID, Message: "主産物が欠損しています"})
	} else if itemID != "" && recipe.MainOutputId != itemID {
		errs = append(errs, GameLoadError{
			FileName: fileName,
			ItemId:   itemID,
			Message:  "主産物がアイテムIDと一致しません: " + recipe.MainOutputId,
		})
	}

	if craft, ok := parseCraftTime(raw.CraftTime.ptr()); ok {
		recipe.CraftTime = craft
	}

	for _, in := range raw.Inputs {
		ref := model.MaterialRef{ItemId: deref(in.ID)}
		if in.ID == nil || ref.ItemId == "" {
			errs = append(errs, GameLoadError{FileName: fileName, ItemId: itemID, Message: "必要素材の参照先IDが欠損しています"})
		}
		if qty, ok := parseQty(in.Qty.ptr()); ok {
			ref.Quantity = qty
		}
		recipe.Inputs = append(recipe.Inputs, ref)
	}
	for _, out := range raw.Outputs {
		ref := model.OutputRef{ItemId: deref(out.ID)}
		if out.ID == nil || ref.ItemId == "" {
			errs = append(errs, GameLoadError{FileName: fileName, ItemId: itemID, Message: "産出物のIDが欠損しています"})
		}
		if qty, ok := parseQty(out.Qty.ptr()); ok {
			ref.Quantity = qty
		}
		recipe.Outputs = append(recipe.Outputs, ref)
	}
	if len(recipe.Outputs) == 0 {
		errs = append(errs, GameLoadError{FileName: fileName, ItemId: itemID, Message: "産出物が欠損しています"})
	} else if recipe.MainOutputId != "" && !outputsContain(recipe.Outputs, recipe.MainOutputId) {
		errs = append(errs, GameLoadError{
			FileName: fileName,
			ItemId:   itemID,
			Message:  "産出物に主産物が含まれていません: " + recipe.MainOutputId,
		})
	}
	return recipe, errs
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func outputsContain(outputs []model.OutputRef, id string) bool {
	for _, out := range outputs {
		if out.ItemId == id {
			return true
		}
	}
	return false
}

func duplicateItemErrors(fileName string, items []parsedItem) []GameLoadError {
	seen := map[string]int{}
	var errs []GameLoadError
	for _, it := range items {
		id := it.item.ItemId
		if id == "" {
			continue
		}
		seen[id]++
		if seen[id] == 2 {
			errs = append(errs, GameLoadError{
				FileName: fileName,
				ItemId:   id,
				Message:  "アイテムIDが重複しています: " + id,
			})
		}
	}
	return errs
}

func duplicateRecipeErrors(fileName string, items []parsedItem) []GameLoadError {
	seen := map[string]int{}
	var errs []GameLoadError
	for _, it := range items {
		for _, recipe := range it.item.Recipes {
			if recipe.RecipeId == "" {
				continue
			}
			seen[recipe.RecipeId]++
			if seen[recipe.RecipeId] == 2 {
				errs = append(errs, GameLoadError{
					FileName: fileName,
					ItemId:   it.item.ItemId,
					Message:  "レシピIDが重複しています: " + recipe.RecipeId,
				})
			}
		}
	}
	return errs
}

func itemIDSet(items []parsedItem) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, it := range items {
		if it.item.ItemId != "" {
			set[it.item.ItemId] = struct{}{}
		}
	}
	return set
}

func referenceErrors(fileName string, items []parsedItem, ids map[string]struct{}) []GameLoadError {
	var errs []GameLoadError
	for _, it := range items {
		for _, recipe := range it.item.Recipes {
			for _, in := range recipe.Inputs {
				if in.ItemId == "" {
					continue
				}
				if _, ok := ids[in.ItemId]; ok {
					continue
				}
				errs = append(errs, GameLoadError{
					FileName:  fileName,
					ItemId:    it.item.ItemId,
					RefItemId: in.ItemId,
					Message:   "必要素材の参照先が存在しません: " + it.item.ItemId + " -> " + in.ItemId,
				})
			}
		}
	}
	return errs
}

func craftTimeErrors(fileName string, nodes []yaml.Node) []GameLoadError {
	var errs []GameLoadError
	for _, node := range nodes {
		var raw rawItem
		if err := node.Decode(&raw); err != nil {
			continue
		}
		itemID := deref(raw.ID)
		for _, recipeNode := range raw.Recipes {
			var recipe rawRecipe
			if err := recipeNode.Decode(&recipe); err != nil {
				continue
			}
			if msg := craftTimeMessage(recipe.CraftTime.ptr()); msg != "" {
				errs = append(errs, GameLoadError{FileName: fileName, ItemId: itemID, Message: msg})
			}
		}
	}
	return errs
}

func craftTimeMessage(n *yaml.Node) string {
	if n == nil || n.Tag == "!!null" || n.Kind == 0 {
		return "生産時間が欠損しています"
	}
	if n.Kind != yaml.ScalarNode || (n.Tag != "!!int" && n.Tag != "!!float") {
		return "生産時間が数値ではありません"
	}
	v, err := strconv.ParseFloat(n.Value, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return "生産時間が数値ではありません"
	}
	if v < minCraftTime || v > maxCraftTime {
		return "生産時間は0.1以上86400以下である必要があります"
	}
	return ""
}

func parseCraftTime(n *yaml.Node) (float64, bool) {
	if craftTimeMessage(n) != "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(n.Value, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func quantityErrors(fileName string, nodes []yaml.Node) []GameLoadError {
	var errs []GameLoadError
	for _, node := range nodes {
		var raw rawItem
		if err := node.Decode(&raw); err != nil {
			continue
		}
		itemID := deref(raw.ID)
		for _, recipeNode := range raw.Recipes {
			var recipe rawRecipe
			if err := recipeNode.Decode(&recipe); err != nil {
				continue
			}
			for _, in := range recipe.Inputs {
				if msg := qtyMessage(in.Qty.ptr(), "必要個数"); msg != "" {
					errs = append(errs, GameLoadError{FileName: fileName, ItemId: itemID, Message: msg})
				}
			}
			for _, out := range recipe.Outputs {
				if msg := qtyMessage(out.Qty.ptr(), "産出数"); msg != "" {
					errs = append(errs, GameLoadError{FileName: fileName, ItemId: itemID, Message: msg})
				}
			}
		}
	}
	return errs
}

func qtyMessage(n *yaml.Node, label string) string {
	if n == nil || n.Tag == "!!null" || n.Kind == 0 {
		return label + "が欠損しています"
	}
	if n.Kind != yaml.ScalarNode || n.Tag != "!!int" {
		return label + "が整数ではありません"
	}
	v, err := strconv.Atoi(n.Value)
	if err != nil {
		wide, wideErr := strconv.ParseInt(n.Value, 10, 64)
		if wideErr != nil {
			return label + "が整数ではありません"
		}
		if wide < minQty || wide > maxQty {
			return label + "は1以上999999以下である必要があります"
		}
		return label + "が整数ではありません"
	}
	if v < minQty || v > maxQty {
		return label + "は1以上999999以下である必要があります"
	}
	return ""
}

func parseQty(n *yaml.Node) (int, bool) {
	if qtyMessage(n, "値") != "" {
		return 0, false
	}
	v, err := strconv.Atoi(n.Value)
	if err != nil {
		return 0, false
	}
	return v, true
}

func lengthAndCountErrors(fileName string, items []parsedItem) []GameLoadError {
	var errs []GameLoadError
	for _, it := range items {
		if it.item.Name != "" {
			n := utf8.RuneCountInString(it.item.Name)
			if n < minNameRunes || n > maxNameRunes {
				errs = append(errs, GameLoadError{
					FileName: fileName,
					ItemId:   it.item.ItemId,
					Message:  "アイテム名は1文字以上100文字以下である必要があります",
				})
			}
		}
		if len(it.item.Recipes) < minRecipes || len(it.item.Recipes) > maxRecipes {
			errs = append(errs, GameLoadError{
				FileName: fileName,
				ItemId:   it.item.ItemId,
				Message:  "レシピ数は1以上20以下である必要があります",
			})
		}
		for _, recipe := range it.item.Recipes {
			if len(recipe.Inputs) > maxInputs {
				errs = append(errs, GameLoadError{
					FileName: fileName,
					ItemId:   it.item.ItemId,
					Message:  "必要素材数は0以上50以下である必要があります",
				})
			}
		}
	}
	return errs
}

func itemsByID(items []parsedItem) map[string]*model.Item {
	stored := make([]model.Item, 0, len(items))
	for _, it := range items {
		if it.item.ItemId == "" {
			continue
		}
		stored = append(stored, it.item)
	}
	out := make(map[string]*model.Item, len(stored))
	for i := range stored {
		if _, exists := out[stored[i].ItemId]; exists {
			continue
		}
		out[stored[i].ItemId] = &stored[i]
	}
	return out
}

func mappingScalar(n *yaml.Node, key string) (string, bool) {
	if n == nil || n.Kind != yaml.MappingNode {
		return "", false
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]
		if k.Value == key && v.Kind == yaml.ScalarNode {
			return v.Value, true
		}
	}
	return "", false
}

// detectCycle は先頭 Recipe の素材依存に有向閉路があれば、
// 閉路を構成する Item_Id 列（先頭と末尾が同一）を返す。閉路がなければ nil。
func detectCycle(items map[string]*model.Item) []string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(items))
	var path []string
	var found []string

	var dfs func(id string)
	dfs = func(id string) {
		if found != nil {
			return
		}
		color[id] = gray
		path = append(path, id)
		if item := items[id]; item != nil {
			if recipe, ok := item.FirstRecipe(); ok {
				for _, in := range recipe.Inputs {
					if found != nil {
						return
					}
					next := in.ItemId
					if _, ok := items[next]; !ok {
						continue
					}
					if color[next] == gray {
						idx := 0
						for i, p := range path {
							if p == next {
								idx = i
								break
							}
						}
						cyc := append([]string{}, path[idx:]...)
						found = append(cyc, next)
						return
					}
					if color[next] == white {
						dfs(next)
					}
				}
			}
		}
		path = path[:len(path)-1]
		color[id] = black
	}

	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if color[id] == white {
			dfs(id)
		}
		if found != nil {
			break
		}
	}
	return found
}
