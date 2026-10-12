// Package calc は生産ツリー構築・素材集計・施設数算出の純粋ロジックである。
// 入力は Data_Loader が検証済みの GameData だけを想定する。
// 複数 Recipe がある Item は常に先頭 Recipe を使う。
package calc

import (
	"fmt"
	"math"
	"sort"

	"game-production-manager/internal/model"
)

// BuildTree は選択 Item を頂点に、先頭 Recipe の素材を再帰展開する。
// ノード数が maxNodes に達したら打ち切り、path 上の再出現は循環として記録して再展開しない。
func BuildTree(g *model.GameData, rootItemId string, maxNodes int) (model.TreeResult, error) {
	if err := requireItem(g, rootItemId); err != nil {
		return model.TreeResult{}, err
	}
	if maxNodes < 0 {
		maxNodes = 0
	}
	b := &treeBuilder{
		g:        g,
		maxNodes: maxNodes,
		nodes:    []model.CyNode{},
		edges:    []model.CyEdge{},
		present:  map[string]bool{},
	}
	b.expand(rootItemId)
	return model.TreeResult{
		Nodes:     b.nodes,
		Edges:     b.edges,
		Truncated: b.truncated,
		Cycle:     b.cycle,
	}, nil
}

type treeBuilder struct {
	g         *model.GameData
	maxNodes  int
	nodes     []model.CyNode
	edges     []model.CyEdge
	present   map[string]bool
	path      []string
	truncated bool
	cycle     []string
}

func (b *treeBuilder) expand(id string) {
	if b.onPath(id) {
		b.recordCycle(id)
		return
	}
	if b.present[id] {
		return
	}
	if len(b.nodes) >= b.maxNodes {
		b.truncated = true
		return
	}
	item, ok := b.g.Items[id]
	if !ok || item == nil {
		return
	}
	b.nodes = append(b.nodes, model.CyNode{
		Data: nodeCardData(b.g, item),
	})
	b.present[id] = true
	b.path = append(b.path, id)
	if recipe, ok := item.FirstRecipe(); ok {
		for _, in := range recipe.Inputs {
			b.link(id, in)
		}
	}
	b.path = b.path[:len(b.path)-1]
}

// nodeCardData は Node_Card 用の表示項目を埋める。集計や施設数の算出は行わない。
// 産出速度は主産物の個数 ÷ 生産時間。生産時間が無いときは OutputRate を付けない。
func nodeCardData(g *model.GameData, item *model.Item) model.CyNodeData {
	data := model.CyNodeData{
		ID:       item.ItemId,
		Label:    item.Name,
		Type:     item.Type,
		Facility: item.Facility,
		Inputs:   []model.CyInput{},
	}
	recipe, ok := item.FirstRecipe()
	if !ok {
		return data
	}
	if qty, ok := recipe.MainOutputQty(); ok {
		data.Output = intPtr(qty)
		if recipe.CraftTime > 0 {
			data.OutputRate = floatPtr(float64(qty) / recipe.CraftTime)
		}
	}
	if len(recipe.Inputs) == 0 {
		return data
	}
	data.HasInputs = true
	inputs := make([]model.CyInput, 0, len(recipe.Inputs))
	for _, in := range recipe.Inputs {
		name := in.ItemId
		if ref := g.Items[in.ItemId]; ref != nil && ref.Name != "" {
			name = ref.Name
		}
		inputs = append(inputs, model.CyInput{Name: name, Qty: in.Quantity})
	}
	data.Inputs = inputs
	return data
}

func intPtr(v int) *int { return &v }

func floatPtr(v float64) *float64 { return &v }

func (b *treeBuilder) link(parent string, in model.MaterialRef) {
	child := in.ItemId
	if child == "" {
		return
	}
	if b.onPath(child) {
		if b.present[child] {
			b.addEdge(parent, child, in.Quantity)
		}
		b.recordCycle(child)
		return
	}
	if !b.present[child] {
		if len(b.nodes) >= b.maxNodes {
			b.truncated = true
			return
		}
		b.expand(child)
	}
	if b.present[child] {
		b.addEdge(parent, child, in.Quantity)
	}
}

func (b *treeBuilder) addEdge(parent, child string, qty int) {
	b.edges = append(b.edges, model.CyEdge{
		Data: model.CyEdgeData{Source: parent, Target: child, Qty: qty},
	})
}

func (b *treeBuilder) onPath(id string) bool {
	for _, p := range b.path {
		if p == id {
			return true
		}
	}
	return false
}

func (b *treeBuilder) recordCycle(reentry string) {
	if b.cycle != nil {
		return
	}
	idx := -1
	for i, id := range b.path {
		if id == reentry {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	cyc := append([]string{}, b.path[idx:]...)
	b.cycle = append(cyc, reentry)
}

// SummarizeMaterials は 1 回の root 生産に必要な各素材の総個数を、経路ごとの積の和で求める。
// root 自身は含めない。循環経路上の Item へは再帰しない。
func SummarizeMaterials(g *model.GameData, rootItemId string, maxNodes int) ([]model.MaterialLine, error) {
	if err := requireItem(g, rootItemId); err != nil {
		return nil, err
	}
	if maxNodes < 0 {
		maxNodes = 0
	}
	summary := map[string]int{}
	expanded := map[string]bool{}
	var walk func(id string, multiplier int, onPath map[string]bool)
	walk = func(id string, multiplier int, onPath map[string]bool) {
		if onPath[id] {
			return
		}
		if !expanded[id] {
			if len(expanded) >= maxNodes {
				return
			}
			expanded[id] = true
		}
		item := g.Items[id]
		if item == nil {
			return
		}
		recipe, ok := item.FirstRecipe()
		if !ok {
			return
		}
		onPath[id] = true
		for _, in := range recipe.Inputs {
			if in.ItemId == "" {
				continue
			}
			summary[in.ItemId] += multiplier * in.Quantity
			if onPath[in.ItemId] {
				continue
			}
			walk(in.ItemId, multiplier*in.Quantity, onPath)
		}
		onPath[id] = false
	}
	walk(rootItemId, 1, map[string]bool{})

	lines := make([]model.MaterialLine, 0, len(summary))
	for id, qty := range summary {
		if qty < 1 {
			continue
		}
		name := id
		var item *model.Item
		if g.Items != nil {
			item = g.Items[id]
		}
		if item != nil && item.Name != "" {
			name = item.Name
		}
		lines = append(lines, model.MaterialLine{
			ItemId:    id,
			Name:      name,
			Quantity:  qty,
			IsPrimary: primaryMaterial(item),
		})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].ItemId < lines[j].ItemId })
	return lines, nil
}

// primaryMaterial は先頭 Recipe の inputs が空かを返す。
// レシピが無い素材も前提素材を持たないものとして 1 次素材とする。必要個数は変えない。
func primaryMaterial(item *model.Item) bool {
	recipe, ok := item.FirstRecipe()
	if !ok {
		return true
	}
	return len(recipe.Inputs) == 0
}

// ComputeFacilities は Target_Rate を素材方向へ再帰展開し、各 Item の必要生産速度と施設数を求める。
// 下流の必要生産速度は切り上げ前の理論値を使い、切り上げは Facility_Count にだけ適用する。
func ComputeFacilities(g *model.GameData, rootItemId string, targetRate float64) (model.FacilityResult, error) {
	if err := requireItem(g, rootItemId); err != nil {
		return model.FacilityResult{}, err
	}
	if targetRate <= 0 || math.IsNaN(targetRate) || math.IsInf(targetRate, 0) {
		return model.FacilityResult{}, fmt.Errorf("目標生産速度が不正です")
	}

	required := map[string]float64{}
	uncomputable := map[string]bool{}
	var order []string
	seen := map[string]bool{}
	var walk func(id string, rate float64, path []string, onPath map[string]bool)
	walk = func(id string, rate float64, path []string, onPath map[string]bool) {
		if onPath[id] {
			markCycle(path, id, uncomputable)
			return
		}
		required[id] += rate
		if !seen[id] {
			seen[id] = true
			order = append(order, id)
		}
		item := g.Items[id]
		if item == nil {
			return
		}
		recipe, ok := item.FirstRecipe()
		if !ok {
			return
		}
		onPath[id] = true
		path = append(path, id)
		for _, in := range recipe.Inputs {
			if in.ItemId == "" {
				continue
			}
			if _, ok := g.Items[in.ItemId]; !ok {
				continue
			}
			// 下流へ渡すのはこの経路の理論値であり、施設数の切り上げ後速度ではない。
			childRate := rate * float64(in.Quantity)
			if onPath[in.ItemId] {
				markCycle(path, in.ItemId, uncomputable)
				continue
			}
			walk(in.ItemId, childRate, path, onPath)
		}
		onPath[id] = false
	}
	walk(rootItemId, targetRate, nil, map[string]bool{})

	items := make([]model.FacilityItem, 0, len(order))
	for _, id := range order {
		item := g.Items[id]
		name := id
		if item != nil && item.Name != "" {
			name = item.Name
		}
		line := model.FacilityItem{
			ItemId:       id,
			Name:         name,
			RequiredRate: required[id],
		}
		if uncomputable[id] {
			line.Uncomputable = true
			line.Reason = "循環により算出できません"
			items = append(items, line)
			continue
		}
		if item == nil {
			return model.FacilityResult{}, fmt.Errorf("アイテムが存在しません: %s", id)
		}
		recipe, ok := item.FirstRecipe()
		if !ok {
			return model.FacilityResult{}, fmt.Errorf("レシピがありません: %s", id)
		}
		qty, ok := recipe.MainOutputQty()
		if !ok || recipe.CraftTime <= 0 {
			return model.FacilityResult{}, fmt.Errorf("産出速度を計算できません: %s", id)
		}
		line.OutputRate = float64(qty) / recipe.CraftTime
		count := int(math.Ceil(line.RequiredRate / line.OutputRate))
		if count < 1 {
			count = 1
		}
		line.FacilityCount = count
		items = append(items, line)
	}
	return model.FacilityResult{Items: items}, nil
}

func markCycle(path []string, reentry string, uncomputable map[string]bool) {
	idx := -1
	for i, id := range path {
		if id == reentry {
			idx = i
			break
		}
	}
	if idx < 0 {
		uncomputable[reentry] = true
		return
	}
	for _, id := range path[idx:] {
		uncomputable[id] = true
	}
	uncomputable[reentry] = true
}

func requireItem(g *model.GameData, itemID string) error {
	if g == nil {
		return fmt.Errorf("ゲームデータがありません")
	}
	if g.Items == nil || g.Items[itemID] == nil {
		return fmt.Errorf("アイテムが存在しません: %s", itemID)
	}
	return nil
}
