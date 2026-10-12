package calc

import (
	"flag"
	"fmt"
	"math"
	"testing"

	"game-production-manager/internal/model"
	"pgregory.net/rapid"
)

func checkAtLeast100(t *testing.T, prop func(*rapid.T)) {
	t.Helper()
	if testing.Short() {
		t.Fatal("プロパティテストは最低100イテレーションが必要なため -short では実行できない")
	}
	f := flag.Lookup("rapid.checks")
	if f == nil {
		t.Fatal("rapid.checks フラグが登録されていない")
	}
	if err := f.Value.Set("100"); err != nil {
		t.Fatal(err)
	}
	rapid.Check(t, prop)
}

type graph struct {
	n     int
	edges [][]int
	qty   [][]int
	craft []float64
	out   []int
}

func (g graph) game() *model.GameData {
	items := make(map[string]*model.Item, g.n)
	for i := 0; i < g.n; i++ {
		id := nodeID(i)
		inputs := make([]model.MaterialRef, 0, len(g.edges[i]))
		for k, to := range g.edges[i] {
			inputs = append(inputs, model.MaterialRef{ItemId: nodeID(to), Quantity: g.qty[i][k]})
		}
		items[id] = item(id, "名前"+id, "中間素材", "施設", g.craft[i], g.out[i], inputs, nil)
	}
	return &model.GameData{GameId: "gen", Name: "gen", Items: items}
}

func nodeID(i int) string { return fmt.Sprintf("n%d", i) }

func drawGraph(t *rapid.T, dag bool) graph {
	n := rapid.IntRange(1, 7).Draw(t, "n")
	g := graph{
		n:     n,
		edges: make([][]int, n),
		qty:   make([][]int, n),
		craft: make([]float64, n),
		out:   make([]int, n),
	}
	crafts := []float64{1, 2, 4}
	outs := []int{1, 2, 4}
	qtys := []int{1, 2, 3}
	for i := 0; i < n; i++ {
		g.craft[i] = crafts[rapid.IntRange(0, len(crafts)-1).Draw(t, fmt.Sprintf("craft_%d", i))]
		g.out[i] = outs[rapid.IntRange(0, len(outs)-1).Draw(t, fmt.Sprintf("out_%d", i))]
		for j := 0; j < n; j++ {
			if dag && j >= i {
				continue
			}
			if !rapid.Bool().Draw(t, fmt.Sprintf("e_%d_%d", i, j)) {
				continue
			}
			g.edges[i] = append(g.edges[i], j)
			g.qty[i] = append(g.qty[i], qtys[rapid.IntRange(0, len(qtys)-1).Draw(t, fmt.Sprintf("q_%d_%d", i, j))])
		}
	}
	return g
}

func TestProperty4TreeTerminatesWithinLimit(t *testing.T) {
	// Feature: game-production-manager, Property 4: ツリー構築は常に停止しノード上限を超えない（循環に対する健全性） — BuildTree は有限ステップで停止し、生成ノード数は maxNodes を超えず、循環があれば経路を記録して循環ノードを再展開しない。
	checkAtLeast100(t, func(t *rapid.T) {
		dag := rapid.Bool().Draw(t, "dag")
		g := drawGraph(t, dag)
		if !dag {
			forceCycle(&g)
		}
		root := rapid.IntRange(0, g.n-1).Draw(t, "root")
		maxNodes := rapid.IntRange(0, g.n+2).Draw(t, "maxNodes")
		game := g.game()

		limited, err := BuildTree(game, nodeID(root), maxNodes)
		if err != nil {
			t.Fatal(err)
		}
		if len(limited.Nodes) > maxNodes {
			t.Fatalf("nodes %d > max %d", len(limited.Nodes), maxNodes)
		}
		if !uniqueNodes(limited) {
			t.Fatalf("ノードが再展開されている: %+v", limited.Nodes)
		}
		if limited.Cycle != nil && !cyclePathValid(limited.Cycle, g) {
			t.Fatalf("循環経路が閉路ではない: %v", limited.Cycle)
		}

		full, err := BuildTree(game, nodeID(root), g.n+5)
		if err != nil {
			t.Fatal(err)
		}
		if len(full.Nodes) > g.n+5 {
			t.Fatalf("full nodes %d", len(full.Nodes))
		}
		if !uniqueNodes(full) {
			t.Fatalf("full で再展開: %+v", full.Nodes)
		}
		if reachableCycle(g, root) {
			if full.Cycle == nil || !cyclePathValid(full.Cycle, g) {
				t.Fatalf("到達可能な循環を記録していない: cycle=%v root=%d edges=%v", full.Cycle, root, g.edges)
			}
		} else if full.Cycle != nil {
			t.Fatalf("到達不能な循環を記録した: %v", full.Cycle)
		}
	})
}

func TestProperty6TreeIsFiniteDAG(t *testing.T) {
	// Feature: game-production-manager, Property 6: 原材料・循環のない Item ではツリーは有限 DAG として展開される — truncated=false のとき各素材 Item は先頭 Recipe の全 Inputs を子に持ち、素材を持たない Item は葉になり、エッジは root から到達可能な素材のみを含む。
	checkAtLeast100(t, func(t *rapid.T) {
		g := drawGraph(t, true)
		root := g.n - 1
		game := g.game()
		tree, err := BuildTree(game, nodeID(root), g.n+5)
		if err != nil {
			t.Fatal(err)
		}
		if tree.Truncated || tree.Cycle != nil {
			t.Fatalf("truncated=%v cycle=%v", tree.Truncated, tree.Cycle)
		}
		reachable := reachableSet(g, root)
		if len(tree.Nodes) != len(reachable) {
			t.Fatalf("nodes %d reachable %d", len(tree.Nodes), len(reachable))
		}
		for _, n := range tree.Nodes {
			idx, ok := parseNode(n.Data.ID, g.n)
			if !ok || !reachable[idx] {
				t.Fatalf("到達不能ノード %s", n.Data.ID)
			}
			if n.Data.Label != "名前"+n.Data.ID || n.Data.Type != "中間素材" {
				t.Fatalf("表示項目が違う: %+v", n.Data)
			}
		}
		got := map[string][]model.CyEdgeData{}
		for _, e := range tree.Edges {
			if _, ok := parseNode(e.Data.Target, g.n); !ok || !reachable[mustParse(e.Data.Target, g.n)] {
				t.Fatalf("到達不能な辺 %+v", e.Data)
			}
			got[e.Data.Source] = append(got[e.Data.Source], e.Data)
		}
		for idx := range reachable {
			id := nodeID(idx)
			edges := got[id]
			if len(edges) != len(g.edges[idx]) {
				t.Fatalf("%s edges=%d inputs=%d", id, len(edges), len(g.edges[idx]))
			}
			for k, to := range g.edges[idx] {
				if edges[k].Target != nodeID(to) || edges[k].Qty != g.qty[idx][k] || edges[k].Source != id {
					t.Fatalf("%s edge %d = %+v", id, k, edges[k])
				}
			}
			if len(g.edges[idx]) == 0 && len(edges) != 0 {
				t.Fatalf("葉 %s に辺がある", id)
			}
		}
	})
}

func TestProperty3MaterialSummaryIsAdditive(t *testing.T) {
	// Feature: game-production-manager, Property 3: 必要素材集計は経路分解に対して加法的 — 各素材の必要個数合計は、その素材へ至る全経路の寄与（経路に沿った必要個数の積）の総和に等しく、同一素材は重複のない 1 行になる。
	checkAtLeast100(t, func(t *rapid.T) {
		g := drawGraph(t, true)
		root := g.n - 1
		lines, err := SummarizeMaterials(g.game(), nodeID(root), g.n+5)
		if err != nil {
			t.Fatal(err)
		}
		want := topoMaterials(g, root)
		got := map[string]int{}
		for _, line := range lines {
			if _, dup := got[line.ItemId]; dup {
				t.Fatalf("重複行 %s", line.ItemId)
			}
			if line.Quantity < 1 || line.Name == "" {
				t.Fatalf("行が不正 %+v", line)
			}
			got[line.ItemId] = line.Quantity
		}
		if _, ok := got[nodeID(root)]; ok {
			t.Fatal("root が素材に含まれている")
		}
		if len(got) != len(want) {
			t.Fatalf("got %v want %v", got, want)
		}
		for id, qty := range want {
			if got[id] != qty {
				t.Fatalf("%s got %d want %d", id, got[id], qty)
			}
		}
	})
}

func TestProperty1FacilityCountIsMinimal(t *testing.T) {
	// Feature: game-production-manager, Property 1: Facility_Count は目標速度を満たす最小台数 — (Facility_Count−1)×Output_Rate < 必要生産速度 ≤ Facility_Count×Output_Rate を満たす 1 以上の整数である。
	checkAtLeast100(t, func(t *rapid.T) {
		g := drawGraph(t, true)
		root := g.n - 1
		rates := []float64{1, 2, 4, 5}
		target := rates[rapid.IntRange(0, len(rates)-1).Draw(t, "rate")]
		res, err := ComputeFacilities(g.game(), nodeID(root), target)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) == 0 || res.Items[0].ItemId != nodeID(root) {
			t.Fatalf("root = %+v", res.Items)
		}
		for _, it := range res.Items {
			if it.Uncomputable || it.FacilityCount < 1 {
				t.Fatalf("算出結果が不正 %+v", it)
			}
			idx := mustParse(it.ItemId, g.n)
			wantOut := float64(g.out[idx]) / g.craft[idx]
			if math.Abs(it.OutputRate-wantOut) > 1e-9 {
				t.Fatalf("output %v want %v", it.OutputRate, wantOut)
			}
			rate := it.RequiredRate
			if it.ItemId == nodeID(root) && math.Abs(rate-target) > 1e-9 {
				t.Fatalf("root rate %v want %v", rate, target)
			}
			left := float64(it.FacilityCount-1) * it.OutputRate
			right := float64(it.FacilityCount) * it.OutputRate
			if !(left < rate+1e-9 && rate <= right+1e-9) {
				t.Fatalf("最小台数ではない: count=%d rate=%v output=%v", it.FacilityCount, rate, it.OutputRate)
			}
		}
	})
}

func TestProperty2RequiredRateIsPathSum(t *testing.T) {
	// Feature: game-production-manager, Property 2: 必要生産速度の再帰展開は親速度と必要個数の積 — 各素材の必要生産速度は、全経路の「親の必要生産速度×1回あたり必要個数」の総和（理論値、切り上げを挟まない）に等しい。
	checkAtLeast100(t, func(t *rapid.T) {
		g := drawGraph(t, true)
		root := g.n - 1
		rates := []float64{1, 2, 4, 5}
		target := rates[rapid.IntRange(0, len(rates)-1).Draw(t, "rate")]
		res, err := ComputeFacilities(g.game(), nodeID(root), target)
		if err != nil {
			t.Fatal(err)
		}
		want := topoRates(g, root, target)
		got := map[string]float64{}
		for _, it := range res.Items {
			if it.Uncomputable {
				t.Fatalf("循環なしなのに算出不可 %+v", it)
			}
			got[it.ItemId] = it.RequiredRate
		}
		if len(got) != len(want) {
			t.Fatalf("got %v want %v", got, want)
		}
		for id, rate := range want {
			if math.Abs(got[id]-rate) > 1e-6 {
				t.Fatalf("%s got %v want %v", id, got[id], rate)
			}
		}
	})
}

func forceCycle(g *graph) {
	if g.n == 1 {
		g.edges[0] = append(g.edges[0], 0)
		g.qty[0] = append(g.qty[0], 1)
		return
	}
	for i := 0; i < g.n; i++ {
		g.edges[i] = append(g.edges[i], (i+1)%g.n)
		g.qty[i] = append(g.qty[i], 1)
	}
}

func uniqueNodes(tree model.TreeResult) bool {
	seen := map[string]bool{}
	for _, n := range tree.Nodes {
		if seen[n.Data.ID] {
			return false
		}
		seen[n.Data.ID] = true
	}
	return true
}

func cyclePathValid(path []string, g graph) bool {
	if len(path) < 2 || path[0] != path[len(path)-1] {
		return false
	}
	has := make([][]bool, g.n)
	for i := range has {
		has[i] = make([]bool, g.n)
		for _, to := range g.edges[i] {
			if to >= 0 && to < g.n {
				has[i][to] = true
			}
		}
	}
	for i := 0; i < len(path)-1; i++ {
		from, ok1 := parseNode(path[i], g.n)
		to, ok2 := parseNode(path[i+1], g.n)
		if !ok1 || !ok2 || !has[from][to] {
			return false
		}
	}
	return true
}

func reachableCycle(g graph, root int) bool {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make([]int, g.n)
	var dfs func(int) bool
	dfs = func(v int) bool {
		color[v] = gray
		for _, next := range g.edges[v] {
			if next < 0 || next >= g.n {
				continue
			}
			if color[next] == gray {
				return true
			}
			if color[next] == white && dfs(next) {
				return true
			}
		}
		color[v] = black
		return false
	}
	return dfs(root)
}

func reachableSet(g graph, root int) map[int]bool {
	seen := map[int]bool{}
	var walk func(int)
	walk = func(v int) {
		if seen[v] {
			return
		}
		seen[v] = true
		for _, next := range g.edges[v] {
			if next >= 0 && next < g.n {
				walk(next)
			}
		}
	}
	walk(root)
	return seen
}

func topoMaterials(g graph, root int) map[string]int {
	mult := make([]int, g.n)
	mult[root] = 1
	summary := map[string]int{}
	for i := g.n - 1; i >= 0; i-- {
		if mult[i] == 0 {
			continue
		}
		for k, to := range g.edges[i] {
			contrib := mult[i] * g.qty[i][k]
			summary[nodeID(to)] += contrib
			mult[to] += contrib
		}
	}
	return summary
}

func topoRates(g graph, root int, target float64) map[string]float64 {
	mult := make([]float64, g.n)
	mult[root] = target
	out := map[string]float64{}
	for i := g.n - 1; i >= 0; i-- {
		if mult[i] == 0 {
			continue
		}
		out[nodeID(i)] = mult[i]
		for k, to := range g.edges[i] {
			mult[to] += mult[i] * float64(g.qty[i][k])
		}
	}
	return out
}

func parseNode(id string, n int) (int, bool) {
	var v int
	if _, err := fmt.Sscanf(id, "n%d", &v); err != nil || v < 0 || v >= n {
		return 0, false
	}
	return v, true
}

func mustParse(id string, n int) int {
	v, ok := parseNode(id, n)
	if !ok {
		panic(id)
	}
	return v
}
