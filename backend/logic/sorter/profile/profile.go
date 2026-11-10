// Package profile reads Sorter V2 / Hive sorting profiles (sorting_profile.json,
// schema_version 1) and answers which category a part in a color belongs in.
//
// A profile carries either a compiled "program" (Hive's current format: ordered
// rules, first match wins) or the older flat "part_to_category" map. Rules in the
// "rules" array describe how Hive built it; the sorter never evaluates them.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	// Misc is the category for pieces no rule takes. It never claims a bin of its
	// own: misc pieces drop into the bucket under the tower unless a bin was
	// assigned misc by hand.
	Misc = "misc"
	// AnyColor stands for an unknown color, and in the flat map for "any color".
	AnyColor = "any_color"

	supportedSchema = 1
)

var ErrInvalid = errors.New("invalid sorting profile")

// Category is one bin-worthy group of pieces.
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Summary describes a profile without its (possibly huge) part map.
type Summary struct {
	Name          string     `json:"name"`
	Description   string     `json:"description,omitempty"`
	HiveID        string     `json:"hive_id,omitempty"`
	ArtifactHash  string     `json:"artifact_hash,omitempty"`
	Format        string     `json:"format"` // "program" or "part_map"
	Categories    []Category `json:"categories"`
	PartCount     int        `json:"part_count"`
	DefaultID     string     `json:"default_category_id"`
	SetBased      bool       `json:"set_based,omitempty"`
	FallbackColor bool       `json:"fallback_by_color,omitempty"`
}

// KitFull reports whether a kit already has enough of a part in a color. nil means
// every kit is still collecting.
type KitFull func(category, part, color string) bool

// Profile is a parsed sorting profile, ready to route pieces.
type Profile struct {
	summary Summary
	names   map[string]string // category id -> name

	program *program
	partMap map[string]string // flat format: "{color}-{part}" -> category
}

// file is the on-disk shape, only the parts the sorter uses.
type file struct {
	ID             string                     `json:"id"`
	Name           string                     `json:"name"`
	Description    string                     `json:"description"`
	SchemaVersion  *int                       `json:"schema_version"`
	Rules          []fileRule                 `json:"rules"`
	Categories     map[string]fileCategory    `json:"categories"`
	DefaultID      string                     `json:"default_category_id"`
	PartToCategory map[string]string          `json:"part_to_category"`
	Program        *fileProgram               `json:"program"`
	ArtifactHash   string                     `json:"artifact_hash"`
	ProfileType    string                     `json:"profile_type"`
	SetInventories map[string]json.RawMessage `json:"set_inventories"`
}

type fileRule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Disabled bool   `json:"disabled"`
}

type fileCategory struct {
	Name string `json:"name"`
}

type fileProgram struct {
	Rules    []programRule `json:"rules"`
	Fallback struct {
		By  string            `json:"by"`
		Map map[string]string `json:"map"`
	} `json:"fallback"`
	Default string `json:"default"`
}

// programRule is a rule ("parts" and "colors", null meaning any) or a kit ("kit":
// part -> colors, a null color meaning any color).
type programRule struct {
	Category string                `json:"category"`
	Parts    *[]string             `json:"parts"`
	Colors   *[]colorID            `json:"colors"`
	Kit      map[string][]*colorID `json:"kit"`
}

// colorID reads a color written as a number or a string.
type colorID string

func (c *colorID) UnmarshalJSON(b []byte) error {
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*c = colorID(n.String())
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*c = colorID(s)
	return nil
}

// Parse reads a profile file's contents.
func Parse(data []byte) (*Profile, error) {
	var f file
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%w: not a JSON profile: %v", ErrInvalid, err)
	}
	if f.SchemaVersion != nil && *f.SchemaVersion != supportedSchema {
		return nil, fmt.Errorf("%w: schema_version %d is not supported (only %d)", ErrInvalid, *f.SchemaVersion, supportedSchema)
	}
	if f.Program == nil && f.PartToCategory == nil {
		return nil, fmt.Errorf("%w: it has neither a program nor a part_to_category map", ErrInvalid)
	}
	name := strings.TrimSpace(f.Name)
	if name == "" {
		name = "Unnamed profile"
	}

	p := &Profile{names: make(map[string]string)}
	for id, c := range f.Categories {
		p.names[id] = c.Name
	}
	for _, r := range f.Rules {
		if _, ok := p.names[r.ID]; !ok && r.ID != "" {
			p.names[r.ID] = r.Name
		}
	}

	p.summary = Summary{
		Name:         name,
		Description:  f.Description,
		HiveID:       f.ID,
		ArtifactHash: f.ArtifactHash,
		SetBased:     f.ProfileType == "set" || len(f.SetInventories) > 0,
	}
	if f.Program != nil {
		prog, err := newProgram(f.Program)
		if err != nil {
			return nil, err
		}
		p.program = prog
		p.summary.Format = "program"
		p.summary.DefaultID = prog.def
		p.summary.PartCount = prog.partCount()
		p.summary.FallbackColor = prog.fallbackBy == "color"
	} else {
		p.partMap = f.PartToCategory
		p.summary.Format = "part_map"
		p.summary.DefaultID = f.DefaultID
		if p.summary.DefaultID == "" {
			p.summary.DefaultID = Misc
		}
		p.summary.PartCount = len(f.PartToCategory)
	}
	p.summary.Categories = p.categoryList(f)
	return p, nil
}

// categoryList is every category a piece can be routed to, in rule order (the order
// "pre-assign from the rules" fills bins), then any others by name. Misc is left
// out: it has no bin of its own.
func (p *Profile) categoryList(f file) []Category {
	seen := map[string]bool{Misc: true}
	var out []Category
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, Category{ID: id, Name: p.CategoryName(id)})
	}
	if p.program != nil {
		for _, r := range p.program.rules {
			add(r.category)
		}
	}
	for _, r := range f.Rules {
		if !r.Disabled {
			add(r.ID)
		}
	}
	var rest []string
	if p.program != nil {
		for _, c := range p.program.fallbackMap {
			rest = append(rest, c)
		}
		rest = append(rest, p.program.def)
	} else {
		for _, c := range p.partMap {
			rest = append(rest, c)
		}
		rest = append(rest, p.summary.DefaultID)
	}
	for id := range p.names {
		rest = append(rest, id)
	}
	sort.Slice(rest, func(i, j int) bool {
		ni, nj := strings.ToLower(p.CategoryName(rest[i])), strings.ToLower(p.CategoryName(rest[j]))
		if ni != nj {
			return ni < nj
		}
		return rest[i] < rest[j]
	})
	for _, id := range rest {
		add(id)
	}
	return out
}

// Summary describes the profile.
func (p *Profile) Summary() Summary {
	return p.summary
}

// CategoryName is a category's display name; generated categories (color_5,
// fallback ids) without one are named from their id.
func (p *Profile) CategoryName(id string) string {
	if name := p.names[id]; name != "" {
		return name
	}
	if id == Misc {
		return "Misc"
	}
	if color, ok := strings.CutPrefix(id, "color_"); ok {
		return "Color " + color
	}
	return id
}

// Route is the category for a part in a color. color may be "" or AnyColor when
// unknown; then only rules for any color can take it.
func (p *Profile) Route(part, color string, kitFull KitFull) string {
	if color == AnyColor {
		color = ""
	}
	if p.program != nil {
		return p.program.route(part, color, kitFull)
	}
	lookupColor := color
	if lookupColor == "" {
		lookupColor = AnyColor
	}
	for _, candidate := range append([]string{part}, BasePartIDs(part)...) {
		for _, key := range []string{lookupColor + "-" + candidate, AnyColor + "-" + candidate} {
			if category, ok := p.partMap[key]; ok {
				return category
			}
		}
	}
	return p.summary.DefaultID
}

type program struct {
	rules       []compiledRule
	fallbackBy  string
	fallbackMap map[string]string
	def         string
}

type compiledRule struct {
	category string
	kit      map[string]map[string]bool // part -> colors ("" = any color)
	parts    map[string]bool            // nil = any part
	colors   map[string]bool            // nil = any color
}

func newProgram(fp *fileProgram) (*program, error) {
	prog := &program{fallbackBy: fp.Fallback.By, fallbackMap: fp.Fallback.Map, def: fp.Default}
	if prog.def == "" {
		prog.def = Misc
	}
	for i, r := range fp.Rules {
		if r.Category == "" {
			return nil, fmt.Errorf("%w: program rule %d has no category", ErrInvalid, i+1)
		}
		rule := compiledRule{category: r.Category}
		if r.Kit != nil {
			rule.kit = make(map[string]map[string]bool, len(r.Kit))
			for part, colors := range r.Kit {
				set := make(map[string]bool, len(colors))
				for _, c := range colors {
					if c == nil {
						set[""] = true
					} else {
						set[string(*c)] = true
					}
				}
				rule.kit[part] = set
			}
		} else {
			if r.Parts != nil {
				rule.parts = make(map[string]bool, len(*r.Parts))
				for _, part := range *r.Parts {
					rule.parts[part] = true
				}
			}
			if r.Colors != nil {
				rule.colors = make(map[string]bool, len(*r.Colors))
				for _, c := range *r.Colors {
					rule.colors[string(c)] = true
				}
			}
		}
		prog.rules = append(prog.rules, rule)
	}
	return prog, nil
}

func (prog *program) partCount() int {
	parts := make(map[string]bool)
	for _, r := range prog.rules {
		for part := range r.parts {
			parts[part] = true
		}
		for part := range r.kit {
			parts[part] = true
		}
	}
	for part := range prog.fallbackMap {
		parts[part] = true
	}
	return len(parts)
}

// route tries the exact part, then each base part. Only the exact part number can
// fill a kit: kits list exact parts.
func (prog *program) route(part, color string, kitFull KitFull) string {
	if category, ok := prog.match(part, color, kitFull, true); ok {
		return category
	}
	for _, base := range BasePartIDs(part) {
		if category, ok := prog.match(base, color, kitFull, false); ok {
			return category
		}
	}
	if prog.fallbackBy == "color" && color != "" {
		return "color_" + color
	}
	return prog.def
}

func (prog *program) match(part, color string, kitFull KitFull, exact bool) (string, bool) {
	for _, r := range prog.rules {
		if r.kit != nil {
			if !exact {
				continue
			}
			wanted, ok := r.kit[part]
			if !ok || !(wanted[color] || wanted[""]) {
				continue
			}
			kitColor := color
			if kitColor == "" {
				kitColor = AnyColor
			}
			if kitFull != nil && kitFull(r.category, part, kitColor) {
				continue
			}
			return r.category, true
		}
		if r.parts == nil {
			// A rule for every part (colors only) says nothing about a variant;
			// only the exact part number may take it.
			if !exact {
				continue
			}
		} else if !r.parts[part] {
			continue
		}
		if r.colors != nil && !r.colors[color] {
			continue
		}
		return r.category, true
	}
	if prog.fallbackBy == "category" {
		if category, ok := prog.fallbackMap[part]; ok {
			return category, true
		}
	}
	return "", false
}

// A part number the profile doesn't list is usually a printed, stickered or
// assembled variant of one it does (3010pb176R, 3464c02, 122c01assy2). Each step
// drops one suffix.
var basePartSteps = []*regexp.Regexp{
	regexp.MustCompile(`assy\d*$`),
	regexp.MustCompile(`(pb|pr|pat|ps)\d+[a-z]?[LR]?$`),
	regexp.MustCompile(`c\d\d$`),
}

// BasePartIDs lists a part's base parts, nearest first: 3010pb176R -> [3010].
func BasePartIDs(part string) []string {
	var bases []string
	current := part
	for {
		next := stripOneSuffix(current)
		if next == "" || next == current {
			return bases
		}
		bases = append(bases, next)
		current = next
	}
}

func stripOneSuffix(part string) string {
	for _, step := range basePartSteps {
		if stripped := step.ReplaceAllString(part, ""); stripped != part && stripped != "" {
			return stripped
		}
	}
	// A trailing lowercase letter after a digit (3068b -> 3068).
	if n := len(part); n >= 2 && part[n-1] >= 'a' && part[n-1] <= 'z' && part[n-2] >= '0' && part[n-2] <= '9' {
		return part[:n-1]
	}
	return part
}
