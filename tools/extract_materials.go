package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Recipe struct {
	Inputs []struct {
		Id  string `yaml:"id"`
		Qty int    `yaml:"qty"`
	} `yaml:"inputs"`
}

type Item struct {
	Id       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Facility string   `yaml:"facility"`
	Recipes  []Recipe `yaml:"recipes"`
}

type GameData struct {
	Name  string `yaml:"name"`
	Items []Item `yaml:"items"`
}

type MaterialItem struct {
	GameId   string `json:"game_id"`
	Id       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Facility string `json:"facility"`
	IsRaw    bool   `json:"is_raw"`
	UsedAsIn bool   `json:"used_as_in"`
}

func main() {
	files, err := filepath.Glob("data/*.yaml")
	if err != nil {
		panic(err)
	}

	gameMaterials := make(map[string][]MaterialItem)

	for _, f := range files {
		gameId := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var g GameData
		if err := yaml.Unmarshal(b, &g); err != nil {
			continue
		}

		usedAsMaterial := make(map[string]int)
		for _, it := range g.Items {
			for _, r := range it.Recipes {
				for _, inp := range r.Inputs {
					usedAsMaterial[inp.Id]++
				}
			}
		}

		var list []MaterialItem
		for _, it := range g.Items {
			isRaw := len(it.Recipes) > 0 && len(it.Recipes[0].Inputs) == 0
			used := usedAsMaterial[it.Id] > 0
			// 素材判定: isRaw または レシピの材料として使われる
			if isRaw || used {
				list = append(list, MaterialItem{
					GameId:   gameId,
					Id:       it.Id,
					Name:     it.Name,
					Type:     it.Type,
					Facility: it.Facility,
					IsRaw:    isRaw,
					UsedAsIn: used,
				})
			}
		}
		sort.Slice(list, func(i, j int) bool {
			return list[i].Id < list[j].Id
		})
		gameMaterials[gameId] = list
		fmt.Printf("%s: %d materials\n", gameId, len(list))
	}

	out, _ := json.MarshalIndent(gameMaterials, "", "  ")
	os.WriteFile("tools/materials.json", out, 0644)
	fmt.Println("Wrote tools/materials.json")
}
