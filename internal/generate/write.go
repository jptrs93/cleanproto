package generate

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

func WriteFiles(outputs []OutputFile) error {
	for _, file := range outputs {
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", filepath.Dir(file.Path), err)
		}
		content := file.Content
		if strings.HasSuffix(file.Path, ".go") {
			formatted, err := format.Source(file.Content)
			if err != nil {
				return fmt.Errorf("gofmt %s: %w", file.Path, err)
			}
			content = formatted
		}
		if err := os.WriteFile(file.Path, content, 0o644); err != nil {
			return fmt.Errorf("write file %s: %w", file.Path, err)
		}
	}
	return nil
}
