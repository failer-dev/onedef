package dartgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type GenerateOptions struct {
	SpecJSON     []byte
	PackageName  string
	OutputDir    string
	ModuleDir    string
	CorePathBase string
}

func GeneratePackage(opts GenerateOptions) error {
	if _, err := exec.LookPath("dart"); err != nil {
		return fmt.Errorf("onedef: dart command not found in PATH")
	}

	tempDir, err := os.MkdirTemp(opts.ModuleDir, ".onedef-sdk-*")
	if err != nil {
		return fmt.Errorf("onedef: failed to create temp sdk dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	specPath := filepath.Join(tempDir, "spec.json")
	if err := os.WriteFile(specPath, opts.SpecJSON, 0o600); err != nil {
		return fmt.Errorf("onedef: failed to write temp spec: %w", err)
	}

	absOut, err := filepath.Abs(opts.OutputDir)
	if err != nil {
		return fmt.Errorf("onedef: failed to resolve sdk output path: %w", err)
	}

	coreDir := filepath.Join(opts.ModuleDir, opts.CorePathBase)
	corePath, err := filepath.Rel(absOut, coreDir)
	if err != nil {
		return fmt.Errorf("onedef: failed to relativize core path: %w", err)
	}

	generatorDir := filepath.Join(opts.ModuleDir, "generators", "dart", "packages", "onedef_gen")
	args := []string{
		"dart",
		"run",
		"bin/onedef_dart_generator.dart",
		"generate",
		"--input", specPath,
		"--out", absOut,
		"--package-name", opts.PackageName,
		"--core-path", corePath,
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = generatorDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("onedef: dart sdk generation failed: %w", err)
	}

	return nil
}
