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
	"runtime"
	"strings"
	"time"
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
	tempDir, err := createTempDir()
	if err != nil {
		log.Fatalf("Error creating temporary directory: %v", err)
	}
	defer func() {
		if err := cleanupTempDir(tempDir); err != nil {
			log.Fatalf("Error cleaning up temporary directory: %v", err)
		}
	}()

	if err := cloneRepo(repoURL, tempDir); err != nil {
		log.Fatalf("Error cloning repository: %v", err)
	}

	goMopPath, err := findGoMod(tempDir)
	if err != nil {
		log.Fatalf("Error finding go.mod: %v", err)
	}

	moduleName, goVersion, err := parseGoMod(goMopPath)
	if err != nil {
		log.Fatalf("Error parsing go.mod: %v", err)
	}

	deps, err := getDependencies(tempDir)
	if err != nil {
		log.Fatalf("Error getting dependencies: %v", err)
	}

	printResults(moduleName, goVersion, deps)
}

func createTempDir() (string, error) {
	if runtime.GOOS == "windows" {
		tempDir := filepath.Join(os.TempDir(), "go-dep-analysis")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			return "", fmt.Errorf("error creating temporary directory: %v", err)
		}
		return tempDir, nil
	}
	return os.MkdirTemp("", "go-dep-analysis")
}

func cleanupTempDir(dir string) error {
	if runtime.GOOS == "windows" {
		for i := 0; i < 3; i++ {
			if err := os.RemoveAll(dir); err == nil {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
		return fmt.Errorf("error removing temporary directory after 3 attempts")
	}
	return os.RemoveAll(dir)
}

func cloneRepo(url, dir string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", "git", "clone", url, dir)
	} else {
		cmd = exec.Command("git", "clone", url, dir)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error cloning repository: %v", err)
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
	if goModPath == "" {
		return "", fmt.Errorf("could not find go.mod in %s", dir)
	}
	return goModPath, err
}

func parseGoMod(goModPath string) (string, string, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", "", fmt.Errorf("error reading go.mod: %v", err)
	}
	data = []byte(strings.ReplaceAll(string(data), "\r\n", "\n"))

	modFile, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return "", "", fmt.Errorf("error parsing go.mod: %v", err)
	}
	if modFile.Module == nil {
		return "", "", fmt.Errorf("module declaration not found")
	}

	goVersion := "unknown"
	if modFile.Go != nil {
		goVersion = modFile.Go.Version
	}
	return modFile.Module.Mod.Path, goVersion, nil
}

func getDependencies(dir string) ([]ModuleInfo, error) {
	if _, err := exec.LookPath("go"); err != nil {
		return nil, fmt.Errorf("go command not found: %v", err)
	}

	cmd := exec.Command("go", "list", "-m", "-u", "-json", "all")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error listing dependencies: %v", err)
	}

	var deps []ModuleInfo
	dec := json.NewDecoder(&out)
	for dec.More() {
		var m ModuleInfo
		if err := dec.Decode(&m); err != nil {
			return nil, fmt.Errorf("error decoding dependencies: %v", err)
		}

		normalizedPath := filepath.ToSlash(m.Path)
		normalizedDir := filepath.ToSlash(dir)
		if !strings.HasPrefix(normalizedPath, ".") && !strings.HasPrefix(normalizedPath, "/") && !strings.Contains(normalizedPath, normalizedDir) {
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
