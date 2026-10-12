package main

import (
	"fmt"
	"os"
	"path/filepath"
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

func main() {
	files, err := filepath.Glob("data/*.yaml")
	if err != nil {
		panic(err)
	}

	for _, f := range files {
		gameId := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var g GameData
		if err := yaml.Unmarshal(b, &g); err != nil {
			fmt.Printf("Error unmarshaling %s: %v\n", f, err)
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

		typeCounts := make(map[string]int)
		rawCount := 0
		materialCount := 0
		for _, it := range g.Items {
			typeCounts[it.Type]++
			isRaw := len(it.Recipes) > 0 && len(it.Recipes[0].Inputs) == 0
			if isRaw {
				rawCount++
			}
			if usedAsMaterial[it.Id] > 0 || isRaw {
				materialCount++
			}
		}

		fmt.Printf("=== %s (%s) ===\n", gameId, g.Name)
		fmt.Printf("  Total: %d, Raw (inputs==0): %d, Material(UsedAsInput or Raw): %d\n", len(g.Items), rawCount, materialCount)
		fmt.Printf("  Types: ")
		for t, c := range typeCounts {
			fmt.Printf("%s=%d ", t, c)
		}
		fmt.Println()
	}
}
