package loader

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// checkAtLeast100 は rapid のイテレーション数を 100 に明示してプロパティを検証する。
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

func TestProperty5CycleDetection(t *testing.T) {
	// Feature: game-production-manager, Property 5: 循環検出の健全性と完全性 — 先頭 Recipe の素材依存グラフに有向閉路が存在するとき、かつそのときに限り Data_Loader は循環エラーを記録し、記録された経路は実際に閉路を構成する Item_Id 列である。
	checkAtLeast100(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 8).Draw(t, "n")
		dag := rapid.Bool().Draw(t, "dag")
		edges := make([][]int, n)
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if dag && j >= i {
					continue
				}
				if rapid.Bool().Draw(t, fmt.Sprintf("e_%d_%d", i, j)) {
					edges[i] = append(edges[i], j)
				}
			}
		}
		if !dag {
			if n == 1 {
				edges[0] = append(edges[0], 0)
			} else {
				for i := 0; i < n; i++ {
					edges[i] = append(edges[i], (i+1)%n)
				}
			}
		}

		dir, err := os.MkdirTemp("", "gpm-p5-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)
		body := graphYAML(n, edges)
		if err := os.WriteFile(filepath.Join(dir, "g.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		res, err := LoadGames(dir)
		if err != nil {
			t.Fatal(err)
		}

		hasCycle := directedHasCycle(n, edges)
		if hasCycle {
			if len(res.Games) != 0 {
				t.Fatalf("閉路があるのにゲームが採用された: %+v", res.Games)
			}
			var cycle []string
			for _, e := range res.Errors {
				if len(e.Cycle) > 0 {
					cycle = e.Cycle
					break
				}
			}
			if !pathIsCycle(cycle, n, edges) {
				t.Fatalf("記録経路が閉路ではない: %v\n%s", cycle, body)
			}
			return
		}
		if len(res.Errors) != 0 || len(res.Games) != 1 {
			t.Fatalf("閉路がないのに失敗した: games=%d errors=%+v\n%s", len(res.Games), res.Errors, body)
		}
	})
}

func graphYAML(n int, edges [][]int) string {
	var b strings.Builder
	b.WriteString("name: \"generated\"\nitems:\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "  - id: n%d\n    name: \"item-%d\"\n    type: \"中間素材\"\n    facility: \"施設\"\n    recipes:\n      - id: n%d_r0\n        craft_time: 1\n        main_output: n%d\n        inputs:", i, i, i, i)
		if len(edges[i]) == 0 {
			b.WriteString(" []\n")
		} else {
			b.WriteString("\n")
			for _, j := range edges[i] {
				fmt.Fprintf(&b, "          - { id: n%d, qty: 1 }\n", j)
			}
		}
		fmt.Fprintf(&b, "        outputs:\n          - { id: n%d, qty: 1 }\n", i)
	}
	return b.String()
}

func directedHasCycle(n int, edges [][]int) bool {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make([]int, n)
	var dfs func(int) bool
	dfs = func(v int) bool {
		color[v] = gray
		for _, next := range edges[v] {
			if next < 0 || next >= n {
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
	for i := 0; i < n; i++ {
		if color[i] == white && dfs(i) {
			return true
		}
	}
	return false
}

func pathIsCycle(path []string, n int, edges [][]int) bool {
	if len(path) < 2 || path[0] != path[len(path)-1] {
		return false
	}
	has := make([][]bool, n)
	for i := range has {
		has[i] = make([]bool, n)
		for _, j := range edges[i] {
			if j >= 0 && j < n {
				has[i][j] = true
			}
		}
	}
	for i := 0; i < len(path)-1; i++ {
		from, ok1 := nodeIndex(path[i], n)
		to, ok2 := nodeIndex(path[i+1], n)
		if !ok1 || !ok2 || !has[from][to] {
			return false
		}
	}
	return true
}

func nodeIndex(id string, n int) (int, bool) {
	var v int
	if _, err := fmt.Sscanf(id, "n%d", &v); err != nil || v < 0 || v >= n {
		return 0, false
	}
	return v, true
}
