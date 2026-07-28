package klipperconfig

import (
	"fmt"
	"sort"
	"strings"
)

// File is a parsed Klipper configuration file.
type File struct {
	Path     string
	Sections map[string]Section
	Order    []string
}

// Section holds key/value options for a config section.
type Section struct {
	Name    string
	Options map[string]string
	Order   []string
}

// Parse reads Klipper INI-style content (sections with `key: value` lines).
func Parse(content string) (*File, error) {
	f := &File{
		Sections: map[string]Section{},
	}

	var current string
	lines := splitLines(content)
	for lineNum, raw := range lines {
		line := stripComment(raw)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.TrimSpace(line[1 : len(line)-1])
			if current == "" {
				return nil, fmt.Errorf("line %d: empty section name", lineNum+1)
			}
			if _, ok := f.Sections[current]; !ok {
				f.Sections[current] = Section{
					Name:    current,
					Options: map[string]string{},
				}
				f.Order = append(f.Order, current)
			}
			continue
		}

		if current == "" {
			return nil, fmt.Errorf("line %d: option outside section", lineNum+1)
		}

		key, value, ok := splitKeyValue(line)
		if !ok {
			return nil, fmt.Errorf("line %d: invalid option %q", lineNum+1, raw)
		}

		sec := f.Sections[current]
		if _, exists := sec.Options[key]; !exists {
			sec.Order = append(sec.Order, key)
		}
		sec.Options[key] = value
		f.Sections[current] = sec
	}

	return f, nil
}

// Get returns a section by name.
func (f *File) Get(section string) (Section, bool) {
	s, ok := f.Sections[section]
	return s, ok
}

// SetOption updates or adds an option in a section, creating the section if needed.
func (f *File) SetOption(section, key, value string) {
	sec, ok := f.Sections[section]
	if !ok {
		sec = Section{Name: section, Options: map[string]string{}}
		f.Sections[section] = sec
		f.Order = append(f.Order, section)
	}
	if _, exists := sec.Options[key]; !exists {
		sec.Order = append(sec.Order, key)
	}
	sec.Options[key] = value
	f.Sections[section] = sec
}

// Serialize renders the file back to Klipper config text.
func (f *File) Serialize() string {
	var b strings.Builder
	for _, sectionName := range f.Order {
		sec := f.Sections[sectionName]
		b.WriteString("[")
		b.WriteString(sec.Name)
		b.WriteString("]\n")
		for _, key := range sec.Order {
			b.WriteString(key)
			b.WriteString(": ")
			b.WriteString(sec.Options[key])
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// SectionNames returns sorted section names (for UI grouping).
func (f *File) SectionNames() []string {
	names := append([]string(nil), f.Order...)
	sort.Strings(names)
	return names
}

// Includes returns include paths from [include] sections and include: keys.
func (f *File) Includes() []string {
	var paths []string
	for _, name := range f.Order {
		if strings.EqualFold(name, "include") {
			sec := f.Sections[name]
			for _, key := range sec.Order {
				paths = append(paths, sec.Options[key])
			}
		}
	}
	if sec, ok := f.Sections["printer"]; ok {
		if v, ok := sec.Options["include"]; ok && v != "" {
			paths = append(paths, v)
		}
	}
	return paths
}

func splitLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return strings.Split(content, "\n")
}

func stripComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func splitKeyValue(line string) (string, string, bool) {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	if key == "" {
		return "", "", false
	}
	return key, value, true
}
