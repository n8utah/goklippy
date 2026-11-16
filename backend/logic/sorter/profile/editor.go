package profile

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Profiles built in the rule editor are stored as a Document: an ordered list of rules,
// first match wins, and what happens to the rest. They route directly from it; nothing
// is compiled ahead.

// FormatEditor is an editor profile's Summary.Format.
const FormatEditor = "editor"

// What happens to pieces no rule takes.
const (
	LeftoverMisc     = "misc"      // the misc bucket
	LeftoverPartType = "part_type" // a category per part type, like the built-in
	LeftoverColor    = "color"     // a category per colour
)

// How a rule splits its pieces into categories.
const (
	SplitNone   = ""
	SplitSize   = "size"   // one per size: Bricks 1 x 2, Bricks 2 x 4, …
	SplitFamily = "family" // one per part type family: Plate, Brick, …
	SplitType   = "type"   // one per exact part type: Plate, Round; Plate, Modified; …
	SplitColor  = "color"  // one per colour
)

const editorVersion = 1

// Document is an editor profile as stored and as the editor edits it.
type Document struct {
	SchemaVersion int    `json:"schema_version"`
	Editor        int    `json:"editor"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Leftover      string `json:"leftover"`
	Rules         []Rule `json:"rules"`
	// HiveID is the Hive profile it came from, if any.
	HiveID string `json:"hive_id,omitempty"`
}

// Rule takes the pieces that match all of its conditions; a condition left empty
// matches anything. A rule with sets is a set rule: it takes only what its sets still
// need, each set into its own category.
type Rule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
	// Category is the category id its pieces go to; empty means the rule's id. Rules
	// converted from Hive keep Hive's ids so bins assigned before keep them.
	Category string `json:"category,omitempty"`

	// Families are part type families ("Plate" takes every plate type); Types are
	// exact part types ("Plate" is only the standard plate, "Plate, Round" the round
	// ones). A piece matches if either takes it.
	Families []string `json:"families,omitempty"`
	Types    []string `json:"types,omitempty"`
	Sizes    []string `json:"sizes,omitempty"` // size keys: "2x4", "1x2x3"
	Parts    []string `json:"parts,omitempty"` // part numbers; variants fall back to these
	Colors   []string `json:"colors,omitempty"`
	Split    string   `json:"split,omitempty"`

	// Kind is KindSets for a set rule, which takes only what its sets need (nothing
	// before a set is added); "" for a rule that matches pieces.
	Kind string    `json:"kind,omitempty"`
	Sets []RuleSet `json:"sets,omitempty"`
}

// KindSets marks a set rule.
const KindSets = "sets"

// RuleSet is one LEGO set (or another fixed list) a set rule collects.
type RuleSet struct {
	Number   string `json:"number"`
	Name     string `json:"name,omitempty"`
	Year     int    `json:"year,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	// Category is its category id; empty means "<rule category>/set:<number>".
	Category string    `json:"category,omitempty"`
	Parts    []KitLine `json:"parts"`
	// Unmapped lists what couldn't be turned into BrickLink part and colour ids.
	Unmapped []string `json:"unmapped,omitempty"`
	// Source is where the parts came from: "viewer" or "rebrickable".
	Source string `json:"source,omitempty"`
}

// IsSetRule reports whether the rule collects sets.
func (r Rule) IsSetRule() bool { return r.Kind == KindSets || len(r.Sets) > 0 }

func (r Rule) category() string {
	if r.Category != "" {
		return r.Category
	}
	return r.ID
}

func (s RuleSet) category(rule Rule) string {
	if s.Category != "" {
		return s.Category
	}
	return rule.category() + "/set:" + s.Number
}

// ParseDocument reads an editor document and tidies it (see Normalize).
func ParseDocument(data []byte) (Document, error) {
	var d Document
	if err := json.Unmarshal(data, &d); err != nil {
		return Document{}, invalidf("not an editor profile: %v", err)
	}
	return d.Normalize()
}

// Normalize checks a document and fills in what's missing: rule ids and names, size
// keys, set categories. It returns the tidied copy.
func (d Document) Normalize() (Document, error) {
	d.SchemaVersion, d.Editor = supportedSchema, editorVersion
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return Document{}, invalidf("the profile needs a name")
	}
	d.Description = strings.TrimSpace(d.Description)
	switch d.Leftover {
	case "":
		d.Leftover = LeftoverMisc
	case LeftoverMisc, LeftoverPartType, LeftoverColor:
	default:
		return Document{}, invalidf("leftover must be misc, part_type or color, not %q", d.Leftover)
	}
	ids := map[string]bool{}
	for _, r := range d.Rules {
		ids[r.ID] = true
	}
	next := 1
	rules := make([]Rule, 0, len(d.Rules))
	seen := map[string]bool{}
	for i, r := range d.Rules {
		r.ID = strings.TrimSpace(r.ID)
		if r.ID == "" || seen[r.ID] || strings.Contains(r.ID, "/") {
			for ids["rule-"+strconv.Itoa(next)] || seen["rule-"+strconv.Itoa(next)] {
				next++
			}
			r.ID = "rule-" + strconv.Itoa(next)
		}
		seen[r.ID] = true
		r.Name = strings.TrimSpace(r.Name)
		if r.Name == "" {
			r.Name = "Rule " + strconv.Itoa(i+1)
		}
		r.Description = strings.TrimSpace(r.Description)
		r.Category = strings.TrimSpace(r.Category)
		r.Families = tidyList(r.Families, nil)
		r.Types = tidyList(r.Types, nil)
		r.Parts = tidyList(r.Parts, nil)
		r.Colors = tidyList(r.Colors, nil)
		var badSize string
		r.Sizes = tidyList(r.Sizes, func(s string) string {
			key := NormalizeSize(s)
			if key == "" && badSize == "" {
				badSize = s
			}
			return key
		})
		if badSize != "" {
			return Document{}, invalidf("rule %q: %q isn't a size (like 2 x 4)", r.Name, badSize)
		}
		switch r.Split {
		case SplitNone, SplitSize, SplitFamily, SplitType, SplitColor:
		default:
			return Document{}, invalidf("rule %q: split must be size, family, type or color, not %q", r.Name, r.Split)
		}
		sets := make([]RuleSet, 0, len(r.Sets))
		setSeen := map[string]bool{}
		for _, s := range r.Sets {
			s.Number = strings.TrimSpace(s.Number)
			if s.Number == "" {
				return Document{}, invalidf("rule %q: a set has no number", r.Name)
			}
			if setSeen[s.Number] {
				continue
			}
			setSeen[s.Number] = true
			s.Name = strings.TrimSpace(s.Name)
			if s.Parts == nil {
				s.Parts = []KitLine{}
			}
			sets = append(sets, s)
		}
		r.Sets = sets
		switch r.Kind {
		case "", KindSets:
		default:
			return Document{}, invalidf("rule %q: kind must be sets or empty, not %q", r.Name, r.Kind)
		}
		if r.IsSetRule() {
			// A set rule takes what its sets need; the other conditions don't apply.
			r.Kind = KindSets
			r.Families, r.Types, r.Sizes, r.Parts, r.Colors, r.Split = nil, nil, nil, nil, nil, SplitNone
		}
		rules = append(rules, r)
	}
	d.Rules = rules
	return d, nil
}

// tidyList trims, maps (when mapper is set; "" drops the item) and de-duplicates,
// keeping the order.
func tidyList(in []string, mapper func(string) string) []string {
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if mapper != nil && s != "" {
			s = mapper(s)
		}
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// editorProfile is a parsed Document, ready to route.
type editorProfile struct {
	doc   Document
	rules []compiledEditorRule
}

type compiledEditorRule struct {
	Rule
	families, types, sizes, parts, colors map[string]bool
	// sets: per set, its category and wanted part -> colors ("" = any colour).
	sets []compiledSet
}

type compiledSet struct {
	category string
	wanted   map[string]map[string]bool
}

func toSet(list []string) map[string]bool {
	if len(list) == 0 {
		return nil
	}
	out := make(map[string]bool, len(list))
	for _, s := range list {
		out[s] = true
	}
	return out
}

func parseEditor(data []byte) (*Profile, error) {
	doc, err := ParseDocument(data)
	if err != nil {
		return nil, err
	}
	return FromDocument(doc)
}

// FromDocument makes a routable profile from an editor document (normalized first).
func FromDocument(doc Document) (*Profile, error) {
	doc, err := doc.Normalize()
	if err != nil {
		return nil, err
	}
	e := &editorProfile{doc: doc}
	p := &Profile{names: map[string]string{}, kits: map[string][]KitLine{}, editor: e}
	parts := map[string]bool{}
	for _, r := range doc.Rules {
		if r.Disabled {
			continue
		}
		c := compiledEditorRule{
			Rule: r, families: toSet(r.Families), types: toSet(r.Types), sizes: toSet(r.Sizes),
			parts: toSet(r.Parts), colors: toSet(r.Colors),
		}
		for _, part := range r.Parts {
			parts[part] = true
		}
		if _, ok := p.names[r.category()]; !ok {
			p.names[r.category()] = r.Name
		}
		for _, s := range r.Sets {
			cat := s.category(r)
			cs := compiledSet{category: cat, wanted: map[string]map[string]bool{}}
			merged := map[[2]string]int{}
			for _, line := range s.Parts {
				if line.Part == "" || line.Need <= 0 {
					continue
				}
				color := line.Color
				if color == AnyColorID || color == AnyColor {
					color = ""
				}
				merged[[2]string{line.Part, color}] += line.Need
				if cs.wanted[line.Part] == nil {
					cs.wanted[line.Part] = map[string]bool{}
				}
				cs.wanted[line.Part][color] = true
				parts[line.Part] = true
			}
			lines := make([]KitLine, 0, len(merged))
			for k, need := range merged {
				lines = append(lines, KitLine{Part: k[0], Color: k[1], Need: need})
			}
			slices.SortFunc(lines, func(a, b KitLine) int {
				if a.Part != b.Part {
					return strings.Compare(a.Part, b.Part)
				}
				return strings.Compare(a.Color, b.Color)
			})
			p.kits[cat] = lines
			name := s.Name
			if s.Category == "" {
				name = strings.TrimSpace(s.Number + " " + s.Name)
			} else if name == "" {
				name = s.Number
			}
			p.names[cat] = name
			c.sets = append(c.sets, cs)
		}
		e.rules = append(e.rules, c)
	}
	p.summary = Summary{
		Name: doc.Name, Description: doc.Description, HiveID: doc.HiveID, Format: FormatEditor,
		PartCount: len(parts), DefaultID: Misc, FallbackColor: doc.Leftover == LeftoverColor,
		Leftover: doc.Leftover, Rules: []RuleInfo{},
	}
	for _, r := range doc.Rules {
		if !r.Disabled {
			p.summary.Rules = append(p.summary.Rules, RuleInfo{ID: r.ID, Name: r.Name, Description: r.Description})
		}
		if r.IsSetRule() && !r.Disabled {
			p.summary.SetBased = true
		}
	}
	p.summary.Categories = e.categories(p)
	return p, nil
}

// Document is the editor document of an editor profile; ok is false for others.
func (p *Profile) Document() (Document, bool) {
	if p.editor == nil {
		return Document{}, false
	}
	return p.editor.doc, true
}

// usualSizes are what a size split without listed sizes pre-assigns bins for.
var usualSizes = []string{"1x1", "1x2", "1x3", "1x4", "1x6", "1x8", "2x2", "2x3", "2x4", "2x6", "2x8"}

// categories lists every category up front, in rule order: what "pre-assign from the
// rules" fills bins with. Split categories not listed are still made as pieces arrive.
func (e *editorProfile) categories(p *Profile) []Category {
	out := []Category{}
	seen := map[string]bool{Misc: true}
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			out = append(out, Category{ID: id, Name: p.CategoryName(id)})
		}
	}
	for _, r := range e.rules {
		if r.IsSetRule() {
			for _, s := range r.sets {
				add(s.category)
			}
			continue
		}
		base := r.category()
		switch r.Split {
		case SplitSize:
			sizes := r.Sizes
			if len(sizes) == 0 {
				sizes = usualSizes
			}
			for _, s := range sizes {
				add(splitID(base, SplitSize, s))
			}
		case SplitFamily:
			families := r.Families
			if len(families) == 0 && len(r.Types) == 0 {
				families = partFamilies
			}
			for _, f := range families {
				add(splitID(base, SplitFamily, f))
			}
			for _, t := range r.Types {
				add(splitID(base, SplitFamily, PartFamily(t)))
			}
		case SplitType:
			for _, t := range r.Types {
				add(splitID(base, SplitType, t))
			}
		}
		add(base)
	}
	if e.doc.Leftover == LeftoverPartType {
		for _, f := range partFamilies {
			add(f)
		}
	}
	return out
}

// splitID is a split category's id: "<rule category>/<kind>:<value>"; colours use the
// colour category's form, "<rule category>/color_<id>".
func splitID(base, kind, value string) string {
	if kind == SplitColor {
		return base + "/" + ColorCategory(value)
	}
	return base + "/" + kind + ":" + value
}

// Match is which rule took a piece, and the category it goes to.
type Match struct {
	Category string `json:"category"`
	RuleID   string `json:"rule_id,omitempty"` // "" when no rule took it
	RuleName string `json:"rule_name,omitempty"`
	// Leftover is true when no rule took it and the profile's leftover choice did.
	Leftover bool `json:"leftover,omitempty"`
}

// Match says which rule takes a piece. For a profile not made in the editor only the
// category is known.
func (p *Profile) Match(piece Piece, kitFull KitFull) Match {
	if p.editor == nil {
		return Match{Category: p.Route(piece, kitFull)}
	}
	return p.editor.match(piece, kitFull)
}

func (e *editorProfile) match(piece Piece, kitFull KitFull) Match {
	color := piece.Color
	if color == AnyColor {
		color = ""
	}
	family := PartFamily(piece.PartCategory)
	size := PartSize(piece.PartName)
	for _, r := range e.rules {
		if r.IsSetRule() {
			for _, s := range r.sets {
				wanted, ok := s.wanted[piece.Part]
				if !ok || !(wanted[color] || wanted[""]) {
					continue
				}
				kitColor := color
				if kitColor == "" {
					kitColor = AnyColor
				}
				if kitFull != nil && kitFull(s.category, piece.Part, kitColor) {
					continue
				}
				return Match{Category: s.category, RuleID: r.ID, RuleName: r.Name}
			}
			continue
		}
		if (r.families != nil || r.types != nil) && !r.families[family] && !r.types[piece.PartCategory] {
			continue
		}
		if r.sizes != nil && !r.sizes[size] {
			continue
		}
		if r.parts != nil && !partListed(r.parts, piece.Part) {
			continue
		}
		if r.colors != nil && !r.colors[color] {
			continue
		}
		return Match{Category: r.splitCategory(piece, family, size, color), RuleID: r.ID, RuleName: r.Name}
	}
	m := Match{Category: Misc, Leftover: true}
	switch e.doc.Leftover {
	case LeftoverPartType:
		if family != "" {
			m.Category = family
		}
	case LeftoverColor:
		if color != "" {
			m.Category = ColorCategory(color)
		}
	}
	return m
}

func partListed(parts map[string]bool, part string) bool {
	if parts[part] {
		return true
	}
	for _, base := range BasePartIDs(part) {
		if parts[base] {
			return true
		}
	}
	return false
}

// splitCategory is the category a matched piece goes to; a piece the split can't place
// (no size in its name, colour unknown) goes to the rule's own category.
func (r compiledEditorRule) splitCategory(piece Piece, family, size, color string) string {
	base := r.category()
	switch r.Split {
	case SplitSize:
		if size != "" {
			return splitID(base, SplitSize, size)
		}
	case SplitFamily:
		if family != "" {
			return splitID(base, SplitFamily, family)
		}
	case SplitType:
		if piece.PartCategory != "" {
			return splitID(base, SplitType, piece.PartCategory)
		}
	case SplitColor:
		if color != "" {
			return splitID(base, SplitColor, color)
		}
	}
	return base
}

// editorCategoryName names a split category: the rule's name and what it split by.
func (p *Profile) editorCategoryName(id string) (string, bool) {
	base, rest, ok := strings.Cut(id, "/")
	if !ok {
		return "", false
	}
	ruleName := p.names[base]
	if ruleName == "" {
		ruleName = base
	}
	if color, ok := strings.CutPrefix(rest, colorPrefix); ok {
		return ruleName + " · Color " + color, true
	}
	kind, value, ok := strings.Cut(rest, ":")
	if !ok {
		return "", false
	}
	switch kind {
	case SplitSize:
		return ruleName + " " + SizeLabel(value), true
	case SplitFamily, SplitType:
		return ruleName + " · " + value, true
	case "set":
		return value, true
	}
	return "", false
}

// SplitColorOf is the colour a colour-split category is for: "rule-2/color_5" -> 5.
func SplitColorOf(id string) (string, bool) {
	_, rest, ok := strings.Cut(id, "/")
	if !ok {
		return "", false
	}
	return strings.CutPrefix(rest, colorPrefix)
}

// accepts reports whether a bin may take a category made as pieces arrive.
func (e *editorProfile) accepts(category string) bool {
	if e.doc.Leftover == LeftoverPartType && !strings.Contains(category, "/") {
		return true
	}
	base, _, ok := strings.Cut(category, "/")
	if !ok {
		return false
	}
	for _, r := range e.rules {
		if r.category() == base && r.Split != SplitNone {
			return true
		}
	}
	return false
}

// DocumentJSON is a document as stored.
func DocumentJSON(doc Document) ([]byte, error) {
	doc, err := doc.Normalize()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encode the profile: %w", err)
	}
	return data, nil
}
