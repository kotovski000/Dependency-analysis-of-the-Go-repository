package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/mod/modfile"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type ModuleInfo struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Update  *struct {
		Path    string `json:"Path"`
		Version string `json:"Version"`
	} `json:"Update,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <git-repo-url>")
		os.Exit(1)
	}

	repoURL := os.Args[1]
	temDir, err := os.MkdirTemp("", "go-dep-analysis")
	if err != nil {
		log.Fatalf("Error creating temporary directory: %v", err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Fatalf("Error removing temporary directory: %v", err)
		}
	}(temDir)

	if err := cloneRepo(repoURL, temDir); err != nil {
		log.Fatalf("Error cloning repository: %v", err)
	}

	goModPath, err := findGoMod(temDir)
	if err != nil {
		log.Fatalf("Error finding go.mod: %v", err)
	}

	moduleName, goVersion, err := parseGoMod(goModPath)
	if err != nil {
		log.Fatalf("Error parsing go.mod: %v", err)
	}

	deps, err := getDependencies(temDir)
	if err != nil {
		log.Fatalf("Error getting dependencies: %v", err)
	}

	printResults(moduleName, goVersion, deps)
}

func cloneRepo(url, dir string) error {
	cmd := exec.Command("git", "clone", url, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func findGoMod(dir string) (string, error) {
	var goModPath string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "go.mod" {
			goModPath = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if goModPath == "" {
		return "", fmt.Errorf("could not find go.mod")
	}
	return goModPath, nil
}

func parseGoMod(goModPath string) (modulePath, goVersion string, err error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", "", fmt.Errorf("error reading go.mod: %v", err)
	}
	modFile, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return "", "", fmt.Errorf("error parsing go.mod: %v", err)
	}

	if modFile.Module == nil {
		return "", "", fmt.Errorf("could not find go.mod")
	}

	return modFile.Module.Mod.Path, modFile.Go.Version, nil
}

func getDependencies(dir string) ([]ModuleInfo, error) {
	cmd := exec.Command("go", "list", "-m", "-u", "-json", "all")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var deps []ModuleInfo
	dec := json.NewDecoder(&out)
	for dec.More() {
		var m ModuleInfo
		if err := dec.Decode(&m); err != nil {
			return nil, err
		}
		if m.Update != nil {
			deps = append(deps, m)
		}
	}
	return deps, nil
}

func printResults(moduleName, goVersion string, deps []ModuleInfo) {
	fmt.Printf("Module: %s\n", moduleName)
	fmt.Printf("Go Module Version: %s\n", goVersion)
	if len(deps) > 0 {
		fmt.Println("Dependencies that can be updated:")
		for _, dep := range deps {
			if dep.Update != nil {
				fmt.Printf("- %s: %s -> %s\n", dep.Path, dep.Version, dep.Update.Version)
			}
		}
	} else {
		fmt.Println("All dependencies are up to date.")
	}
}
