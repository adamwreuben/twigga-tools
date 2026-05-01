package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// getExportedFunctions uses a temporary Node.js script to read the user's index.js
// and extract the names of all exported functions.
func GetExportedFunctions(dir string) ([]string, error) {
	scriptPath := filepath.Join(dir, ".twigga_sniffer.js")

	scriptContent := `
		try {
			const userCode = require('./index.js');
			const exports = Object.keys(userCode);
			if (exports.length > 0) {
				console.log(exports.join(','));
			}
		} catch(e) {
			console.error("Failed to parse functions:", e.message);
			process.exit(1);
		}
	`

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		return nil, err
	}

	defer os.Remove(scriptPath)

	cmd := exec.Command("node", ".twigga_sniffer.js")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read exports. Ensure your index.js has no syntax errors. Error: %v", string(out))
	}

	result := strings.TrimSpace(string(out))
	if result == "" {
		return nil, nil // No functions exported
	}

	return strings.Split(result, ","), nil
}
