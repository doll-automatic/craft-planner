package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	b, err := os.ReadFile("tools/materials.json")
	if err != nil {
		panic(err)
	}
	var allMat map[string][]struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	json.Unmarshal(b, &allMat)

	manifestBytes, err := os.ReadFile("C:\\Users\\Doll\\.cursor\\projects\\c-Users-Doll-Documents-MyPrograms-AI\\agent-tools\\290d5051-12d8-4991-8885-2f78a020c2fa.txt")
	if err != nil {
		panic(err)
	}
	var manifest map[string]string
	json.Unmarshal(manifestBytes, &manifest)

	factorioMats := allMat["factorio"]
	fmt.Printf("Total factorio materials to match: %d\n", len(factorioMats))

	matched := 0
	var missing []string
	downloadMap := make(map[string]string) // itemId -> url

	for _, m := range factorioMats {
		// try direct keys
		keysToTry := []string{
			"item_" + m.Id,
			"fluid_" + m.Id,
			"entity_" + m.Id,
			"recipe_" + m.Id,
			m.Id,
		}
		found := false
		for _, k := range keysToTry {
			if url, ok := manifest[k]; ok {
				downloadMap[m.Id] = url
				matched++
				found = true
				break
			}
		}
		if !found {
			// fuzzy check
			for mk, url := range manifest {
				if strings.HasSuffix(mk, "_"+m.Id) {
					downloadMap[m.Id] = url
					matched++
					found = true
					break
				}
			}
		}
		if !found {
			missing = append(missing, m.Id)
		}
	}

	fmt.Printf("Matched: %d / %d\n", matched, len(factorioMats))
	if len(missing) > 0 {
		fmt.Printf("Missing (%d): %v\n", len(missing), missing)
	}

	out, _ := json.MarshalIndent(downloadMap, "", "  ")
	os.WriteFile("tools/factorio_download_map.json", out, 0644)
}
