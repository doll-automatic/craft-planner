// Package model はゲームデータと算出結果のドメインモデルを定義する。
package model

// GameData は 1 ゲーム分の Item 定義集合である。
// Items のキーは Item_Id。internal/loader の検証を通過したものだけを internal/calc へ渡す。
type GameData struct {
	GameId string           `json:"gameId" yaml:"-"`
	Name   string           `json:"name" yaml:"name"`
	Items  map[string]*Item `json:"items" yaml:"-"`
}

// Item は生産対象または素材の定義である。
// ツリー展開・集計・施設数算出は先頭 Recipe（Recipes[0]）だけを使う。
type Item struct {
	ItemId   string   `json:"id" yaml:"id"`
	Name     string   `json:"name" yaml:"name"`
	Type     string   `json:"type" yaml:"type"`
	Facility string   `json:"facility" yaml:"facility"`
	Recipes  []Recipe `json:"recipes" yaml:"recipes"`
}

// FirstRecipe は算出に使う先頭 Recipe を返す。
// レシピ選択は本バージョンの対象外であり、常に先頭で固定する。
func (it *Item) FirstRecipe() (*Recipe, bool) {
	if it == nil || len(it.Recipes) == 0 {
		return nil, false
	}
	return &it.Recipes[0], true
}

// Recipe は 1 つの生産レシピ（入力素材と産出物の組）である。
type Recipe struct {
	RecipeId     string        `json:"id" yaml:"id"`
	CraftTime    float64       `json:"craftTime" yaml:"craft_time"`
	Inputs       []MaterialRef `json:"inputs" yaml:"inputs"`
	Outputs      []OutputRef   `json:"outputs" yaml:"outputs"`
	MainOutputId string        `json:"mainOutput" yaml:"main_output"`
}

// MainOutputQty は主産物の産出数を返す。
func (r *Recipe) MainOutputQty() (int, bool) {
	if r == nil {
		return 0, false
	}
	for _, out := range r.Outputs {
		if out.ItemId == r.MainOutputId {
			return out.Quantity, true
		}
	}
	return 0, false
}

// MaterialRef は必要素材の参照（他 Item への Item_Id と必要個数）である。
type MaterialRef struct {
	ItemId   string `json:"id" yaml:"id"`
	Quantity int    `json:"qty" yaml:"qty"`
}

// OutputRef は産出物（主産物または副産物）である。
type OutputRef struct {
	ItemId   string `json:"id" yaml:"id"`
	Quantity int    `json:"qty" yaml:"qty"`
}

// CyNode は Cytoscape.js の node 要素である。
type CyNode struct {
	Data CyNodeData `json:"data"`
}

// CyInput はツリーのノードカード下段に出す前提素材（Item 名へ解決済み）である。
// Qty は 1 回の生産に必要な直接個数で、素材合計とは別である。
type CyInput struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

// CyNodeData はツリーのノードカードの描画に必要な表示項目を持つ。
// Output と OutputRate は生産レシピが無い、または生産時間が無いとき nil（JSON では省略）になる。
type CyNodeData struct {
	ID         string    `json:"id"`
	Label      string    `json:"label"`
	Type       string    `json:"type"`
	Output     *int      `json:"output,omitempty"`
	Facility   string    `json:"facility"`
	OutputRate *float64  `json:"outputRate,omitempty"`
	Inputs     []CyInput `json:"inputs"`
	HasInputs  bool      `json:"hasInputs"`
}

// CyEdge は消費側（source）から素材側（target）への有向辺である。
type CyEdge struct {
	Data CyEdgeData `json:"data"`
}

// CyEdgeData は 1 回あたりの必要個数 qty を持つ。
type CyEdgeData struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Qty    int    `json:"qty"`
}

// TreeResult は生産ツリーの描画用結果である。
// Cycle が nil のとき JSON では null になる。
type TreeResult struct {
	Nodes     []CyNode `json:"nodes"`
	Edges     []CyEdge `json:"edges"`
	Truncated bool     `json:"truncated"`
	Cycle     []string `json:"cycle"`
}

// MaterialLine は素材名と必要個数の合計 1 行である。
// Quantity は、ツリー取得では root 1 個あたり、施設数の取得では目標速度での 1 秒あたりの個数を表す。
// どちらも切り上げ前の理論値で、端数を持つことがある。整数への切り上げは表示時にだけ行う。
// IsPrimary は先頭 Recipe の inputs が空の 1 次素材を示す。必要個数の合計とは独立した表示用フラグである。
type MaterialLine struct {
	ItemId    string  `json:"itemId"`
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
	IsPrimary bool    `json:"isPrimary"`
}

// FacilityItem は 1 つの Item の必要速度と施設数である。
// RequiredQuantity は必要速度（個/秒）の切り上げ前の理論値である（名前は個数だが、JSON の互換のため変えていない）。
// FacilityCount はそれを 1 台あたり産出速度（OutputRate、個/秒）で割って切り上げた台数である。
type FacilityItem struct {
	ItemId           string  `json:"itemId"`
	Name             string  `json:"name"`
	RequiredQuantity float64 `json:"requiredQuantity"`
	OutputRate       float64 `json:"outputRate"`
	FacilityCount    int     `json:"facilityCount"`
	Uncomputable     bool    `json:"uncomputable"`
	Reason           string  `json:"reason,omitempty"`
}

// FacilityResult は目標速度を素材方向へ展開した結果である。
type FacilityResult struct {
	Items []FacilityItem `json:"items"`
}
