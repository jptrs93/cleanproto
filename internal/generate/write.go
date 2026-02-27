package generate

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteFiles(outputs []OutputFile) error {
	for _, file := range outputs {
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", filepath.Dir(file.Path), err)
		}
		if err := os.WriteFile(file.Path, file.Content, 0o644); err != nil {
			return fmt.Errorf("write file %s: %w", file.Path, err)
		}
	}
	return nil
}
