package profile

import (
	"slices"
	"sort"
	"strconv"
	"strings"
)

// EditableDocument is a profile as an editor document: an editor profile's own, or a
// built-in or Hive profile turned into rules that route the same way. Converted rules
// keep the profile's category ids, so bins already given them keep them.
func (p *Profile) EditableDocument() Document {
	if p.editor != nil {
		return p.editor.doc
	}
	doc := Document{Name: p.summary.Name, Description: p.summary.Description, HiveID: p.summary.HiveID, Leftover: LeftoverMisc, Rules: []Rule{}}
	switch {
	case p.builtin == BuiltinPartType:
		doc.Leftover = LeftoverPartType
	case p.builtin == BuiltinColor:
		doc.Leftover = LeftoverColor
	case p.program != nil:
		p.convertProgram(&doc)
	default:
		p.convertPartMap(&doc)
	}
	return doc
}

func (p *Profile) newRule(doc *Document, category string) Rule {
	return Rule{ID: "rule-" + strconv.Itoa(len(doc.Rules)+1), Name: p.CategoryName(category), Category: category}
}

func (p *Profile) convertProgram(doc *Document) {
	prog := p.program
	for _, r := range prog.rules {
		rule := p.newRule(doc, r.category)
		if r.kit != nil {
			set := RuleSet{Number: r.category, Name: p.CategoryName(r.category), Category: r.category}
			if lines, ok := p.kits[r.category]; ok {
				set.Parts = append([]KitLine(nil), lines...)
			} else {
				for part, colors := range r.kit {
					for c := range colors {
						set.Parts = append(set.Parts, KitLine{Part: part, Color: c, Need: 1})
					}
				}
				sortLines(set.Parts)
			}
			rule.Kind, rule.Sets = KindSets, []RuleSet{set}
		} else {
			rule.Parts = sortedKeys(r.parts)
			rule.Colors = sortedKeys(r.colors)
		}
		doc.Rules = append(doc.Rules, rule)
	}
	if prog.fallbackBy == "category" && len(prog.fallbackMap) > 0 {
		byCategory := map[string][]string{}
		for part, c := range prog.fallbackMap {
			byCategory[c] = append(byCategory[c], part)
		}
		for _, c := range p.sortedByName(byCategory) {
			rule := p.newRule(doc, c)
			rule.Parts = byCategory[c]
			sort.Strings(rule.Parts)
			doc.Rules = append(doc.Rules, rule)
		}
	}
	if prog.fallbackBy == "color" {
		doc.Leftover = LeftoverColor
	}
	if prog.def != "" && prog.def != Misc {
		doc.Rules = append(doc.Rules, p.newRule(doc, prog.def))
	}
}

// convertPartMap turns the flat "{color}-{part}" map into a rule per category and
// colour.
func (p *Profile) convertPartMap(doc *Document) {
	type key struct{ category, color string }
	groups := map[key][]string{}
	for k, c := range p.partMap {
		color, part, ok := strings.Cut(k, "-")
		if !ok {
			continue
		}
		if color == AnyColor {
			color = ""
		}
		groups[key{c, color}] = append(groups[key{c, color}], part)
	}
	keys := make([]key, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ni, nj := strings.ToLower(p.CategoryName(keys[i].category)), strings.ToLower(p.CategoryName(keys[j].category))
		if ni != nj {
			return ni < nj
		}
		// A colour-specific rule goes before the any-colour one for the same category.
		if (keys[i].color == "") != (keys[j].color == "") {
			return keys[j].color == ""
		}
		return keys[i].color < keys[j].color
	})
	// Colour-specific entries win over any-colour ones in the map; keep that by putting
	// every colour-specific rule first.
	sort.SliceStable(keys, func(i, j int) bool { return keys[i].color != "" && keys[j].color == "" })
	for _, k := range keys {
		rule := p.newRule(doc, k.category)
		rule.Parts = groups[k]
		sort.Strings(rule.Parts)
		if k.color != "" {
			rule.Colors = []string{k.color}
		}
		doc.Rules = append(doc.Rules, rule)
	}
	if def := p.summary.DefaultID; def != "" && def != Misc {
		doc.Rules = append(doc.Rules, p.newRule(doc, def))
	}
}

func (p *Profile) sortedByName(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.SortFunc(out, func(a, b string) int {
		return strings.Compare(strings.ToLower(p.CategoryName(a)), strings.ToLower(p.CategoryName(b)))
	})
	return out
}

func sortedKeys(m map[string]bool) []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortLines(lines []KitLine) {
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Part != lines[j].Part {
			return lines[i].Part < lines[j].Part
		}
		return lines[i].Color < lines[j].Color
	})
}
