package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAutoInstallCandidate(t *testing.T) {
	t.Run("python alias", func(t *testing.T) {
		candidate := DetectAutoInstallCandidate(".py", "ModuleNotFoundError: No module named 'Crypto.Hash'", t.TempDir())
		if candidate == nil {
			t.Fatal("expected python candidate")
		}
		if candidate.Manager != "python" || candidate.PackageName != "pycryptodome" {
			t.Fatalf("unexpected python candidate: %+v", candidate)
		}
	})

	t.Run("node package", func(t *testing.T) {
		candidate := DetectAutoInstallCandidate(".js", "Error: Cannot find module 'axios'", t.TempDir())
		if candidate == nil {
			t.Fatal("expected node candidate")
		}
		if candidate.Manager != "nodejs" || candidate.PackageName != "axios" {
			t.Fatalf("unexpected node candidate: %+v", candidate)
		}
	})

	t.Run("node relative module ignored", func(t *testing.T) {
		candidate := DetectAutoInstallCandidate(".js", "Error: Cannot find module './local-helper'", t.TempDir())
		if candidate != nil {
			t.Fatalf("expected nil candidate, got %+v", candidate)
		}
	})

	t.Run("go module requires go.mod", func(t *testing.T) {
		workDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte("module example.com/demo\n\ngo 1.25\n"), 0644); err != nil {
			t.Fatalf("write go.mod: %v", err)
		}
		candidate := DetectAutoInstallCandidate(".go", "main.go:5:2: no required module provides package github.com/gin-gonic/gin; to add it:\n\tgo get github.com/gin-gonic/gin", workDir)
		if candidate == nil {
			t.Fatal("expected go candidate")
		}
		if candidate.Manager != "go" || candidate.PackageName != "github.com/gin-gonic/gin" {
			t.Fatalf("unexpected go candidate: %+v", candidate)
		}
	})

	t.Run("go without module manifest is ignored", func(t *testing.T) {
		candidate := DetectAutoInstallCandidate(".go", "main.go:5:2: no required module provides package github.com/gin-gonic/gin", t.TempDir())
		if candidate != nil {
			t.Fatalf("expected nil candidate, got %+v", candidate)
		}
	})
}
