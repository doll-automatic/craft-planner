package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFiles(t *testing.T, files map[string]string) LoadResult {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := LoadGames(dir)
	if err != nil {
		t.Fatalf("LoadGames: %v", err)
	}
	return res
}

func assertNoErrors(t *testing.T, res LoadResult) {
	t.Helper()
	if len(res.Errors) != 0 {
		t.Fatalf("errors = %+v", res.Errors)
	}
}

func assertErrorContains(t *testing.T, res LoadResult, parts ...string) {
	t.Helper()
	for _, err := range res.Errors {
		blob := err.FileName + " " + err.ItemId + " " + err.RefItemId + " " + strings.Join(err.Cycle, " ") + " " + err.Message
		ok := true
		for _, part := range parts {
			if !strings.Contains(blob, part) {
				ok = false
				break
			}
		}
		if ok {
			return
		}
	}
	t.Fatalf("errors %+v に %q が含まれない", res.Errors, parts)
}

const schemaYAML = `
# ゲーム名（表示用）。コメントは Data_Loader が無視する
name: "Sample Factory Game"

items:
  # 原材料: 必要素材を持たない（Inputs が空）
  - id: iron_ore
    name: "鉄鉱石"
    type: "中間素材"
    facility: "採掘機"
    recipes:
      - id: iron_ore_r0
        craft_time: 2.0
        main_output: iron_ore
        inputs: []            # 原材料のため空
        outputs:
          - { id: iron_ore, qty: 1 }

  # 中間素材: 鉄鉱石から鉄板を生産
  - id: iron_plate
    name: "鉄板"
    type: "中間素材"
    facility: "製錬炉"
    recipes:
      - id: iron_plate_r0
        craft_time: 3.2
        main_output: iron_plate
        inputs:
          - { id: iron_ore, qty: 1 }
        outputs:
          - { id: iron_plate, qty: 1 }

  # 副産物を持つ例
  - id: gear
    name: "歯車"
    type: "中間素材"
    facility: "組立機"
    recipes:
      - id: gear_r0
        craft_time: 1.0
        main_output: gear
        inputs:
          - { id: iron_plate, qty: 2 }
        outputs:
          - { id: gear, qty: 1 }       # 主産物
          - { id: scrap, qty: 1 }      # 副産物
`

func TestSchemaExampleWithComments(t *testing.T) {
	res := loadFiles(t, map[string]string{"sample.yaml": schemaYAML})
	assertNoErrors(t, res)
	if len(res.Games) != 1 {
		t.Fatalf("games = %d", len(res.Games))
	}
	g := res.Games[0]
	if g.GameId != "sample" || g.Name != "Sample Factory Game" {
		t.Fatalf("game = %+v", g)
	}
	if len(g.Items) != 3 {
		t.Fatalf("items = %d", len(g.Items))
	}

	ore := g.Items["iron_ore"]
	if ore == nil || ore.Name != "鉄鉱石" || ore.Facility != "採掘機" || ore.Type != "中間素材" {
		t.Fatalf("ore = %+v", ore)
	}
	if len(ore.Recipes) != 1 || len(ore.Recipes[0].Inputs) != 0 || ore.Recipes[0].CraftTime != 2 {
		t.Fatalf("ore recipe = %+v", ore.Recipes)
	}

	plate := g.Items["iron_plate"]
	if plate == nil || plate.Recipes[0].CraftTime != 3.2 || len(plate.Recipes[0].Inputs) != 1 {
		t.Fatalf("plate = %+v", plate)
	}
	if plate.Recipes[0].Inputs[0].ItemId != "iron_ore" || plate.Recipes[0].Inputs[0].Quantity != 1 {
		t.Fatalf("plate input = %+v", plate.Recipes[0].Inputs)
	}

	gear := g.Items["gear"]
	if gear == nil || gear.Recipes[0].MainOutputId != "gear" {
		t.Fatalf("gear = %+v", gear)
	}
	if len(gear.Recipes[0].Outputs) != 2 || gear.Recipes[0].Outputs[1].ItemId != "scrap" || gear.Recipes[0].Outputs[1].Quantity != 1 {
		t.Fatalf("outputs = %+v", gear.Recipes[0].Outputs)
	}
	if _, ok := g.Items["scrap"]; ok {
		t.Fatal("副産物 scrap は Item として定義されていない")
	}
}

func TestSyntaxErrorIsolatedFromOtherFiles(t *testing.T) {
	good := `
name: "良いゲーム"
items:
  - id: a
    name: "素材"
    type: "中間素材"
    facility: "手"
    recipes:
      - id: a_r
        craft_time: 1
        main_output: a
        inputs: []
        outputs:
          - { id: a, qty: 1 }
`
	res := loadFiles(t, map[string]string{
		"bad.yaml":  "items: [\n  - :\n",
		"good.yaml": good,
		"note.txt":  "これはYAMLではない",
	})
	if len(res.Games) != 1 || res.Games[0].GameId != "good" {
		t.Fatalf("games = %+v", res.Games)
	}
	assertErrorContains(t, res, "bad.yaml", "構文")
}

func TestYMLExtensionAndRawMaterial(t *testing.T) {
	body := `
name: "木工房"
items:
  - id: log
    name: "原木"
    type: "中間素材"
    facility: "伐採機"
    recipes:
      - id: log_r
        craft_time: 0.1
        main_output: log
        inputs: []
        outputs:
          - { id: log, qty: 999999 }
`
	res := loadFiles(t, map[string]string{"wood.yml": body})
	assertNoErrors(t, res)
	if len(res.Games) != 1 || res.Games[0].GameId != "wood" {
		t.Fatalf("games = %+v", res.Games)
	}
	logItem := res.Games[0].Items["log"]
	if logItem == nil || len(logItem.Recipes[0].Inputs) != 0 || logItem.Recipes[0].Outputs[0].Quantity != 999999 {
		t.Fatalf("log = %+v", logItem)
	}
	if logItem.Recipes[0].CraftTime != 0.1 {
		t.Fatalf("craft = %v", logItem.Recipes[0].CraftTime)
	}
}

func TestReferenceError(t *testing.T) {
	body := `
name: "参照切れ"
items:
  - id: plate
    name: "板"
    type: "中間素材"
    facility: "炉"
    recipes:
      - id: plate_r
        craft_time: 1
        main_output: plate
        inputs:
          - { id: missing_ore, qty: 1 }
        outputs:
          - { id: plate, qty: 1 }
`
	res := loadFiles(t, map[string]string{"ref.yaml": body})
	if len(res.Games) != 0 {
		t.Fatalf("games = %d", len(res.Games))
	}
	assertErrorContains(t, res, "ref.yaml", "plate", "missing_ore", "参照先が存在しません")
}

func TestCraftTimeErrors(t *testing.T) {
	cases := []struct {
		name string
		time string
		want string
	}{
		{"too small", "0.09", "0.1以上86400以下"},
		{"too large", "86400.1", "0.1以上86400以下"},
		{"not number", "\"abc\"", "数値ではありません"},
		{"missing", "", "欠損"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			craftLine := ""
			if tc.time != "" {
				craftLine = "        craft_time: " + tc.time + "\n"
			}
			body := "name: \"t\"\nitems:\n  - id: a\n    name: \"A\"\n    type: \"中間素材\"\n    facility: \"f\"\n    recipes:\n      - id: a_r\n" + craftLine + "        main_output: a\n        inputs: []\n        outputs:\n          - { id: a, qty: 1 }\n"
			res := loadFiles(t, map[string]string{"t.yaml": body})
			if len(res.Games) != 0 {
				t.Fatal("ゲームが採用されている")
			}
			assertErrorContains(t, res, "t.yaml", "a", tc.want)
		})
	}
}

func TestQuantityErrors(t *testing.T) {
	cases := []struct {
		name string
		qty  string
		want string
	}{
		{"zero", "0", "1以上999999以下"},
		{"negative", "-3", "1以上999999以下"},
		{"float", "1.5", "整数ではありません"},
		{"too big", "1000000", "1以上999999以下"},
		{"missing", "", "欠損"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			qtyLine := ""
			if tc.qty != "" {
				qtyLine = "qty: " + tc.qty
			}
			body := "name: \"t\"\nitems:\n  - id: ore\n    name: \"鉱石\"\n    type: \"中間素材\"\n    facility: \"f\"\n    recipes:\n      - id: ore_r\n        craft_time: 1\n        main_output: ore\n        inputs: []\n        outputs:\n          - { id: ore, " + qtyLine + " }\n"
			res := loadFiles(t, map[string]string{"q.yaml": body})
			if len(res.Games) != 0 {
				t.Fatal("ゲームが採用されている")
			}
			assertErrorContains(t, res, "q.yaml", "ore", tc.want)
		})
	}
}

func TestDuplicateItemID(t *testing.T) {
	body := `
name: "重複"
items:
  - id: dup
    name: "一つ目"
    type: "武器"
    facility: "作業台"
    recipes:
      - id: dup_r1
        craft_time: 1
        main_output: dup
        inputs: []
        outputs:
          - { id: dup, qty: 1 }
  - id: dup
    name: "二つ目"
    type: "武器"
    facility: "作業台"
    recipes:
      - id: dup_r2
        craft_time: 1
        main_output: dup
        inputs: []
        outputs:
          - { id: dup, qty: 1 }
`
	res := loadFiles(t, map[string]string{"dup.yaml": body})
	if len(res.Games) != 0 {
		t.Fatal("重複ゲームが採用されている")
	}
	assertErrorContains(t, res, "dup.yaml", "dup", "重複")
}

func TestCycleRecorded(t *testing.T) {
	body := `
name: "循環"
items:
  - id: a
    name: "A"
    type: "中間素材"
    facility: "f"
    recipes:
      - id: a_r
        craft_time: 1
        main_output: a
        inputs:
          - { id: b, qty: 1 }
        outputs:
          - { id: a, qty: 1 }
  - id: b
    name: "B"
    type: "中間素材"
    facility: "f"
    recipes:
      - id: b_r
        craft_time: 1
        main_output: b
        inputs:
          - { id: a, qty: 2 }
        outputs:
          - { id: b, qty: 1 }
`
	res := loadFiles(t, map[string]string{"cycle.yaml": body})
	if len(res.Games) != 0 {
		t.Fatal("循環ゲームが採用されている")
	}
	var cycle []string
	for _, err := range res.Errors {
		if len(err.Cycle) > 0 {
			cycle = err.Cycle
			break
		}
	}
	if !isCycle(cycle, map[string][]string{"a": {"b"}, "b": {"a"}}) {
		t.Fatalf("cycle = %v errors=%+v", cycle, res.Errors)
	}
	assertErrorContains(t, res, "cycle.yaml", "循環")
}

func TestNameRecipeAndInputLimits(t *testing.T) {
	t.Run("long name", func(t *testing.T) {
		name := strings.Repeat("あ", 101)
		body := "name: \"t\"\nitems:\n  - id: a\n    name: \"" + name + "\"\n    type: \"中間素材\"\n    facility: \"f\"\n    recipes:\n      - id: a_r\n        craft_time: 1\n        main_output: a\n        inputs: []\n        outputs:\n          - { id: a, qty: 1 }\n"
		res := loadFiles(t, map[string]string{"n.yaml": body})
		assertErrorContains(t, res, "n.yaml", "a", "100文字")
	})
	t.Run("too many recipes", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("name: \"t\"\nitems:\n  - id: a\n    name: \"A\"\n    type: \"中間素材\"\n    facility: \"f\"\n    recipes:\n")
		for i := 0; i < 21; i++ {
			b.WriteString("      - id: r" + itoa(i) + "\n        craft_time: 1\n        main_output: a\n        inputs: []\n        outputs:\n          - { id: a, qty: 1 }\n")
		}
		res := loadFiles(t, map[string]string{"r.yaml": b.String()})
		assertErrorContains(t, res, "r.yaml", "a", "レシピ数")
	})
	t.Run("too many inputs", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("name: \"t\"\nitems:\n  - id: ore\n    name: \"鉱\"\n    type: \"中間素材\"\n    facility: \"f\"\n    recipes:\n      - id: ore_r\n        craft_time: 1\n        main_output: ore\n        inputs: []\n        outputs:\n          - { id: ore, qty: 1 }\n  - id: a\n    name: \"A\"\n    type: \"中間素材\"\n    facility: \"f\"\n    recipes:\n      - id: a_r\n        craft_time: 1\n        main_output: a\n        inputs:\n")
		for i := 0; i < 51; i++ {
			b.WriteString("          - { id: ore, qty: 1 }\n")
		}
		b.WriteString("        outputs:\n          - { id: a, qty: 1 }\n")
		res := loadFiles(t, map[string]string{"in.yaml": b.String()})
		assertErrorContains(t, res, "in.yaml", "a", "必要素材数")
	})
}

func TestMainOutputMismatch(t *testing.T) {
	body := `
name: "主産物"
items:
  - id: a
    name: "A"
    type: "武器"
    facility: "f"
    recipes:
      - id: a_r
        craft_time: 86400
        main_output: other
        inputs: []
        outputs:
          - { id: other, qty: 1 }
`
	res := loadFiles(t, map[string]string{"m.yaml": body})
	assertErrorContains(t, res, "m.yaml", "a", "主産物")
}

func TestMissingDataDir(t *testing.T) {
	_, err := LoadGames(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("存在しないディレクトリで error が返らなかった")
	}
}

func TestEmptyDir(t *testing.T) {
	res, err := LoadGames(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Games) != 0 || len(res.Errors) != 0 {
		t.Fatalf("%+v", res)
	}
}

func itoa(n int) string {
	return strings.TrimPrefix(strings.ReplaceAll(strings.Repeat(" ", 0), "", ""), "") + strconvItoa(n)
}

func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func isCycle(path []string, edges map[string][]string) bool {
	if len(path) < 2 || path[0] != path[len(path)-1] {
		return false
	}
	for i := 0; i < len(path)-1; i++ {
		ok := false
		for _, next := range edges[path[i]] {
			if next == path[i+1] {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
