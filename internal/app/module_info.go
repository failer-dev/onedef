package app

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type moduleInfo struct {
	Path string
	Dir  string
}

func currentModuleInfo() (moduleInfo, error) {
	cmd := exec.Command("go", "list", "-m", "-json")
	output, err := cmd.Output()
	if err != nil {
		return moduleInfo{}, fmt.Errorf("onedef: failed to resolve current module: %w", err)
	}

	var info moduleInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return moduleInfo{}, fmt.Errorf("onedef: failed to parse current module info: %w", err)
	}
	if info.Path == "" || info.Dir == "" {
		return moduleInfo{}, fmt.Errorf("onedef: current module info is incomplete")
	}
	return info, nil
}
