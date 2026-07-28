package klipperconfig

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LoadFile reads and parses a Klipper config from disk.
func LoadFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	f, err := Parse(string(data))
	if err != nil {
		return nil, err
	}
	f.Path = path
	return f, nil
}

// SaveFile writes a config file, creating a .bak backup if the target exists.
func SaveFile(f *File, path string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}
	content := f.Serialize()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		backup := path + "." + time.Now().Format("20060102-150405") + ".bak"
		if err := copyFile(path, backup); err != nil {
			return fmt.Errorf("backup config: %w", err)
		}
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	f.Path = path
	return nil
}

// ResolveIncludes merges included files into a single File (one level deep).
func ResolveIncludes(root *File, read func(path string) (*File, error)) (*File, error) {
	merged := &File{
		Path:     root.Path,
		Sections: map[string]Section{},
		Order:    []string{},
	}

	merge := func(f *File) {
		for _, name := range f.Order {
			src := f.Sections[name]
			if strings.EqualFold(name, "include") {
				continue
			}
			dst, ok := merged.Sections[name]
			if !ok {
				dst = Section{Name: name, Options: map[string]string{}}
				merged.Sections[name] = dst
				merged.Order = append(merged.Order, name)
			}
			for _, key := range src.Order {
				if _, exists := dst.Options[key]; !exists {
					dst.Order = append(dst.Order, key)
				}
				dst.Options[key] = src.Options[key]
			}
			merged.Sections[name] = dst
		}
	}

	merge(root)

	baseDir := filepath.Dir(root.Path)
	for _, inc := range root.Includes() {
		incPath := inc
		if !filepath.IsAbs(incPath) {
			incPath = filepath.Join(baseDir, inc)
		}
		incPath = filepath.Clean(incPath)
		child, err := read(incPath)
		if err != nil {
			return nil, fmt.Errorf("include %q: %w", inc, err)
		}
		merge(child)
	}

	return merged, nil
}

// ValidatePath ensures path is under root (local-only safety).
func ValidatePath(root, path string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return err
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path %q is outside config root", path)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// ListConfigFiles returns .cfg files in a directory (non-recursive).
func ListConfigFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".cfg") {
			files = append(files, e.Name())
		}
	}
	sortStrings(files)
	return files, nil
}

func sortStrings(items []string) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j] < items[i] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}
