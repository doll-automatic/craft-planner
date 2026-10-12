package calc

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"game-production-manager/internal/model"
)

func item(id, name, typ, facility string, craft float64, outQty int, inputs []model.MaterialRef, extra []model.OutputRef) *model.Item {
	outputs := []model.OutputRef{{ItemId: id, Quantity: outQty}}
	outputs = append(outputs, extra...)
	if inputs == nil {
		inputs = []model.MaterialRef{}
	}
	return &model.Item{
		ItemId:   id,
		Name:     name,
		Type:     typ,
		Facility: facility,
		Recipes: []model.Recipe{{
			RecipeId:     id + "_r0",
			CraftTime:    craft,
			MainOutputId: id,
			Inputs:       inputs,
			Outputs:      outputs,
		}},
	}
}

func gearGame() *model.GameData {
	ore := item("iron_ore", "鉄鉱石", "中間素材", "採掘機", 2, 1, nil, nil)
	plate := item("iron_plate", "鉄板", "中間素材", "製錬炉", 3.2, 1, []model.MaterialRef{{ItemId: "iron_ore", Quantity: 1}}, nil)
	gear := item("gear", "歯車", "中間素材", "組立機", 1, 1, []model.MaterialRef{{ItemId: "iron_plate", Quantity: 2}}, []model.OutputRef{{ItemId: "scrap", Quantity: 1}})
	// 先頭以外の Recipe は算出に使わない。
	gear.Recipes = append(gear.Recipes, model.Recipe{
		RecipeId:     "gear_alt",
		CraftTime:    10,
		MainOutputId: "gear",
		Inputs:       []model.MaterialRef{{ItemId: "iron_ore", Quantity: 99}},
		Outputs:      []model.OutputRef{{ItemId: "gear", Quantity: 1}},
	})
	return &model.GameData{
		GameId: "sample",
		Name:   "Sample Factory Game",
		Items: map[string]*model.Item{
			"iron_ore":   ore,
			"iron_plate": plate,
			"gear":       gear,
		},
	}
}

func facilityByID(res model.FacilityResult, id string) (model.FacilityItem, bool) {
	for _, it := range res.Items {
		if it.ItemId == id {
			return it, true
		}
	}
	return model.FacilityItem{}, false
}

func TestGearExampleFacilitiesAndMaterials(t *testing.T) {
	g := gearGame()
	res, err := ComputeFacilities(g, "gear", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) == 0 || res.Items[0].ItemId != "gear" {
		t.Fatalf("root = %+v", res.Items)
	}

	gear, ok := facilityByID(res, "gear")
	if !ok || gear.FacilityCount != 2 || gear.RequiredRate != 2 || gear.OutputRate != 1 {
		t.Fatalf("gear = %+v", gear)
	}
	plate, ok := facilityByID(res, "iron_plate")
	if !ok || plate.FacilityCount != 13 || math.Abs(plate.RequiredRate-4) > 1e-9 {
		t.Fatalf("plate = %+v", plate)
	}
	if math.Abs(plate.OutputRate-(1/3.2)) > 1e-9 {
		t.Fatalf("plate output = %v", plate.OutputRate)
	}
	ore, ok := facilityByID(res, "iron_ore")
	if !ok || ore.FacilityCount != 8 || math.Abs(ore.RequiredRate-4) > 1e-9 || ore.OutputRate != 0.5 {
		t.Fatalf("ore = %+v", ore)
	}

	lines, err := SummarizeMaterials(g, "gear", 1000)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	primary := map[string]bool{}
	for _, line := range lines {
		got[line.ItemId] = line.Quantity
		primary[line.ItemId] = line.IsPrimary
		if line.Quantity < 1 {
			t.Fatalf("qty = %+v", line)
		}
	}
	if got["iron_plate"] != 2 || got["iron_ore"] != 2 || len(got) != 2 {
		t.Fatalf("materials = %+v", lines)
	}
	if primary["iron_plate"] || !primary["iron_ore"] {
		t.Fatalf("isPrimary = %+v", lines)
	}

	tree, err := BuildTree(g, "gear", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Truncated || tree.Cycle != nil || len(tree.Nodes) != 3 {
		t.Fatalf("tree = %+v", tree)
	}
	if tree.Nodes[0].Data.Label != "歯車" || tree.Nodes[0].Data.Type != "中間素材" {
		t.Fatalf("node = %+v", tree.Nodes[0])
	}
	gearNode := nodeData(t, tree, "gear")
	if gearNode.Facility != "組立機" || !gearNode.HasInputs || gearNode.Output == nil || *gearNode.Output != 1 {
		t.Fatalf("gear card = %+v", gearNode)
	}
	if gearNode.OutputRate == nil || *gearNode.OutputRate != 1 {
		t.Fatalf("gear rate = %+v", gearNode.OutputRate)
	}
	if len(gearNode.Inputs) != 1 || gearNode.Inputs[0].Name != "鉄板" || gearNode.Inputs[0].Qty != 2 {
		t.Fatalf("gear inputs = %+v", gearNode.Inputs)
	}
	plateNode := nodeData(t, tree, "iron_plate")
	if plateNode.OutputRate == nil || math.Abs(*plateNode.OutputRate-(1/3.2)) > 1e-9 {
		t.Fatalf("plate card rate = %+v", plateNode.OutputRate)
	}
	oreNode := nodeData(t, tree, "iron_ore")
	if oreNode.HasInputs || len(oreNode.Inputs) != 0 || oreNode.Facility != "採掘機" {
		t.Fatalf("ore card = %+v", oreNode)
	}
}

func TestRawMaterialIsSingleNode(t *testing.T) {
	g := gearGame()
	tree, err := BuildTree(g, "iron_ore", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Nodes) != 1 || len(tree.Edges) != 0 || tree.Truncated || tree.Cycle != nil {
		t.Fatalf("tree = %+v", tree)
	}
	ore := tree.Nodes[0].Data
	if ore.HasInputs || len(ore.Inputs) != 0 || ore.Output == nil || *ore.Output != 1 || ore.OutputRate == nil || *ore.OutputRate != 0.5 {
		t.Fatalf("raw card = %+v", ore)
	}
	lines, err := SummarizeMaterials(g, "iron_ore", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Fatalf("materials = %+v", lines)
	}
	res, err := ComputeFacilities(g, "iron_ore", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].FacilityCount != 4 {
		t.Fatalf("facilities = %+v", res.Items)
	}
}

func TestTheoreticalRateIgnoresFacilityRounding(t *testing.T) {
	// 親は 3 秒で 2 個。目標 1 個/秒だと施設数は ceil(1.5)=2 で実効速度は 4/3。
	// 下流の必要速度は理論値 1 のままである。
	parent := item("parent", "親", "中間素材", "組立機", 3, 2, []model.MaterialRef{{ItemId: "child", Quantity: 1}}, nil)
	child := item("child", "子", "中間素材", "炉", 1, 1, nil, nil)
	g := &model.GameData{Items: map[string]*model.Item{"parent": parent, "child": child}}
	res, err := ComputeFacilities(g, "parent", 1)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := facilityByID(res, "parent")
	c, _ := facilityByID(res, "child")
	if p.FacilityCount != 2 {
		t.Fatalf("parent = %+v", p)
	}
	if math.Abs(c.RequiredRate-1) > 1e-9 {
		t.Fatalf("child rate = %v, want 1 (not effective 4/3)", c.RequiredRate)
	}
}

func TestCycleStopsFacilityComputation(t *testing.T) {
	a := item("a", "A", "中間素材", "f", 1, 1, []model.MaterialRef{{ItemId: "b", Quantity: 1}}, nil)
	b := item("b", "B", "中間素材", "f", 1, 1, []model.MaterialRef{{ItemId: "a", Quantity: 1}}, nil)
	g := &model.GameData{Items: map[string]*model.Item{"a": a, "b": b}}
	res, err := ComputeFacilities(g, "a", 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range res.Items {
		if !it.Uncomputable {
			t.Fatalf("循環なのに算出されている: %+v", it)
		}
	}
	tree, err := BuildTree(g, "a", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Cycle) < 2 || tree.Cycle[0] != tree.Cycle[len(tree.Cycle)-1] {
		t.Fatalf("cycle = %v", tree.Cycle)
	}
	ids := map[string]int{}
	for _, n := range tree.Nodes {
		ids[n.Data.ID]++
	}
	if ids["a"] != 1 || ids["b"] != 1 {
		t.Fatalf("nodes re-expanded: %+v", tree.Nodes)
	}
}

func nodeData(t *testing.T, tree model.TreeResult, id string) model.CyNodeData {
	t.Helper()
	for _, n := range tree.Nodes {
		if n.Data.ID == id {
			return n.Data
		}
	}
	t.Fatalf("node %s が無い", id)
	return model.CyNodeData{}
}

func TestNodeCardOmitsRateWithoutCraftTime(t *testing.T) {
	wood := &model.Item{
		ItemId: "wood", Name: "木材", Type: "原材料", Facility: "手作業",
		Recipes: []model.Recipe{{
			RecipeId: "wood_r", CraftTime: 0, MainOutputId: "wood",
			Outputs: []model.OutputRef{{ItemId: "wood", Quantity: 1}},
		}},
	}
	bare := &model.Item{ItemId: "bare", Name: "未加工", Type: "その他", Facility: "なし"}
	parent := item("parent", "親", "中間素材", "組立機", 2, 3, []model.MaterialRef{{ItemId: "missing", Quantity: 4}}, nil)
	g := &model.GameData{Items: map[string]*model.Item{
		"wood": wood, "bare": bare, "parent": parent,
	}}

	tree, err := BuildTree(g, "wood", 1000)
	if err != nil {
		t.Fatal(err)
	}
	card := tree.Nodes[0].Data
	if card.HasInputs || len(card.Inputs) != 0 || card.OutputRate != nil || card.Output == nil || *card.Output != 1 || card.Facility != "手作業" {
		t.Fatalf("wood = %+v", card)
	}
	raw, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"outputRate"`)) || !bytes.Contains(raw, []byte(`"inputs":[]`)) {
		t.Fatalf("json = %s", raw)
	}

	bareTree, err := BuildTree(g, "bare", 1000)
	if err != nil {
		t.Fatal(err)
	}
	bareCard := bareTree.Nodes[0].Data
	if bareCard.Output != nil || bareCard.OutputRate != nil || bareCard.HasInputs {
		t.Fatalf("bare = %+v", bareCard)
	}

	parentTree, err := BuildTree(g, "parent", 1000)
	if err != nil {
		t.Fatal(err)
	}
	parentCard := nodeData(t, parentTree, "parent")
	if !parentCard.HasInputs || len(parentCard.Inputs) != 1 || parentCard.Inputs[0].Name != "missing" || parentCard.Inputs[0].Qty != 4 {
		t.Fatalf("parent inputs = %+v", parentCard.Inputs)
	}
	if parentCard.Output == nil || *parentCard.Output != 3 || parentCard.OutputRate == nil || *parentCard.OutputRate != 1.5 {
		t.Fatalf("parent card = %+v rate=%v", parentCard, parentCard.OutputRate)
	}
}

func TestInvalidTargetRate(t *testing.T) {
	g := gearGame()
	if _, err := ComputeFacilities(g, "gear", 0); err == nil {
		t.Fatal("0 を受け付けた")
	}
	if _, err := ComputeFacilities(g, "gear", -1); err == nil {
		t.Fatal("負数を受け付けた")
	}
	if _, err := ComputeFacilities(g, "missing", 1); err == nil {
		t.Fatal("存在しない Item を受け付けた")
	}
}
