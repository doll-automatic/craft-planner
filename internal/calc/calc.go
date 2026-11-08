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

// craftsFor は quantity 個の産出に要するレシピの実行回数（切り上げ前の理論値）を返す。
// 主産物の産出数が分からないレシピは 1 回 1 個として扱う。
func craftsFor(recipe *model.Recipe, quantity float64) float64 {
	if qty, ok := recipe.MainOutputQty(); ok && qty > 0 {
		return quantity / float64(qty)
	}
	return quantity
}

// SummarizeMaterials は root を 1 個作るのに必要な各素材の総個数を、経路ごとの寄与の和で求める。
// 寄与は「親の必要個数 ÷ 親の産出数 × 1 回あたり必要個数」の積み重ねで、端数を切り上げない。
// 各 Item の内訳は 1 回だけ計算して再利用する。root 自身は含めない。循環の先へは進まない。
// 目標速度への倍率は ScaleMaterials で掛ける。
func SummarizeMaterials(g *model.GameData, rootItemId string, maxNodes int) ([]model.MaterialLine, error) {
	if err := requireItem(g, rootItemId); err != nil {
		return nil, err
	}
	if maxNodes < 0 {
		maxNodes = 0
	}
	w := &materialWalk{
		g:        g,
		maxNodes: maxNodes,
		memo:     map[string]map[string]float64{},
		expanded: map[string]bool{},
		stack:    map[string]bool{},
	}
	summary := w.ingredients(rootItemId)

	lines := make([]model.MaterialLine, 0, len(summary))
	for id, qty := range summary {
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

// materialWalk は「1 個作るときの素材」を Item ごとに記憶する。
type materialWalk struct {
	g        *model.GameData
	maxNodes int
	memo     map[string]map[string]float64
	expanded map[string]bool
	stack    map[string]bool
}

func (w *materialWalk) ingredients(id string) map[string]float64 {
	if m, ok := w.memo[id]; ok {
		return m
	}
	if w.stack[id] {
		return map[string]float64{}
	}
	if !w.expanded[id] {
		if len(w.expanded) >= w.maxNodes {
			return map[string]float64{}
		}
		w.expanded[id] = true
	}
	out := map[string]float64{}
	item := w.g.Items[id]
	recipe, ok := (*model.Recipe)(nil), false
	if item != nil {
		recipe, ok = item.FirstRecipe()
	}
	if !ok {
		w.memo[id] = out
		return out
	}
	w.stack[id] = true
	crafts := craftsFor(recipe, 1)
	for _, in := range recipe.Inputs {
		if in.ItemId == "" {
			continue
		}
		q := crafts * float64(in.Quantity)
		out[in.ItemId] += q
		if w.stack[in.ItemId] {
			continue
		}
		for childID, childQty := range w.ingredients(in.ItemId) {
			out[childID] += q * childQty
		}
	}
	w.stack[id] = false
	w.memo[id] = out
	return out
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

// ScaleMaterials は 1 個分の素材合計を目標速度（個/秒）倍する。
// 段ごとに切り上げず、理論値のまま掛ける。整数への切り上げは表示側で行う。
func ScaleMaterials(lines []model.MaterialLine, targetRate float64) ([]model.MaterialLine, error) {
	if err := requireTarget(targetRate); err != nil {
		return nil, err
	}
	out := make([]model.MaterialLine, 0, len(lines))
	for _, line := range lines {
		line.Quantity *= targetRate
		if math.IsInf(line.Quantity, 0) || math.IsNaN(line.Quantity) {
			return nil, fmt.Errorf("必要個数が大きすぎます")
		}
		out = append(out, line)
	}
	return out, nil
}

// ComputeFacilities は目標速度（個/秒）を素材方向へ展開し、各 Item の必要速度と施設数を求める。
// 施設数は必要速度 ÷ 1台あたり産出速度で、切り上げは FacilityCount の表示値にだけ行う。
// 各 Item の内訳は 1 回だけ計算する。下流へ渡す値は切り上げ前の理論値である。
func ComputeFacilities(g *model.GameData, rootItemId string, targetRate float64) (model.FacilityResult, error) {
	if err := requireItem(g, rootItemId); err != nil {
		return model.FacilityResult{}, err
	}
	if err := requireTarget(targetRate); err != nil {
		return model.FacilityResult{}, err
	}

	w := &facilityWalk{
		g:     g,
		memo:  map[string]facilityUnit{},
		stack: map[string]bool{},
	}
	perRoot, cyc := w.unit(rootItemId)
	required := map[string]float64{}
	uncomputable := map[string]bool{}
	if cyc {
		for id := range perRoot {
			uncomputable[id] = true
		}
	}
	order := make([]string, 0, len(perRoot))
	for id, qty := range perRoot {
		required[id] = qty * targetRate
		order = append(order, id)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i] == rootItemId {
			return true
		}
		if order[j] == rootItemId {
			return false
		}
		return order[i] < order[j]
	})

	items := make([]model.FacilityItem, 0, len(order))
	for _, id := range order {
		item := g.Items[id]
		name := id
		if item != nil && item.Name != "" {
			name = item.Name
		}
		line := model.FacilityItem{
			ItemId:           id,
			Name:             name,
			RequiredQuantity: required[id],
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
			line.Uncomputable = true
			line.Reason = "生産時間がないため算出できません"
			items = append(items, line)
			continue
		}
		line.OutputRate = float64(qty) / recipe.CraftTime
		line.FacilityCount = ceilUnits(line.RequiredQuantity / line.OutputRate)
		items = append(items, line)
	}
	return model.FacilityResult{Items: items}, nil
}

type facilityUnit struct {
	need  map[string]float64
	cycle bool
}

// facilityWalk は「1 個/秒あたり、自分を含む必要速度」を Item ごとに記憶する。
type facilityWalk struct {
	g     *model.GameData
	memo  map[string]facilityUnit
	stack map[string]bool
}

func (w *facilityWalk) unit(id string) (map[string]float64, bool) {
	if m, ok := w.memo[id]; ok {
		return m.need, m.cycle
	}
	if w.stack[id] {
		return map[string]float64{id: 1}, true
	}
	need := map[string]float64{id: 1}
	item := w.g.Items[id]
	if item == nil {
		w.memo[id] = facilityUnit{need: need}
		return need, false
	}
	recipe, ok := item.FirstRecipe()
	if !ok {
		w.memo[id] = facilityUnit{need: need}
		return need, false
	}
	w.stack[id] = true
	cycle := false
	crafts := craftsFor(recipe, 1)
	for _, in := range recipe.Inputs {
		if in.ItemId == "" {
			continue
		}
		if _, known := w.g.Items[in.ItemId]; !known {
			continue
		}
		q := crafts * float64(in.Quantity)
		sub, subCycle := w.unit(in.ItemId)
		if subCycle {
			cycle = true
		}
		for childID, childQty := range sub {
			need[childID] += q * childQty
		}
	}
	w.stack[id] = false
	w.memo[id] = facilityUnit{need: need, cycle: cycle}
	return need, cycle
}

// ceilUnits は施設数の切り上げである。二進小数の誤差で 1 台増えないよう、ごく小さい余りは切り捨てる。
func ceilUnits(x float64) int {
	n := int(math.Ceil(x - 1e-9))
	if n < 1 {
		return 1
	}
	return n
}

func requireTarget(rate float64) error {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 || rate > 1_000_000_000 {
		return fmt.Errorf("目標速度が不正です")
	}
	return nil
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
