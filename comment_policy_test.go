package godis_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"testing"
)

func TestGoSourcesContainNoComments(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		if len(file.Comments) != 0 {
			t.Errorf("%s contains %d comment groups", path, len(file.Comments))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
