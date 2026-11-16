package profile

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartSize(t *testing.T) {
	cases := map[string]string{
		"Brick 2 x 4":         "2x4",
		"Brick 4 x 2":         "2x4", // the same size either way round
		"Plate 1 x 6":         "1x6",
		"Brick 1 x 2 x 3":     "1x2x3", // its own size
		"Brick, Round  1 x 1": "1x1",
		"Plate, Modified 2 x 2 x 2/3 with 2 Studs on Side": "2x2x2/3",
		"Technic, Pin with Friction Ridges":                "",
	}
	for name, want := range cases {
		assert.Equal(t, want, PartSize(name), name)
	}
	assert.Equal(t, "2x4", NormalizeSize("4 X 2"))
	assert.Equal(t, "", NormalizeSize("big"))
	assert.Equal(t, "2 x 4", SizeLabel("2x4"))
}

func mustDoc(t *testing.T, doc Document) *Profile {
	t.Helper()
	p, err := FromDocument(doc)
	require.NoError(t, err)
	return p
}

func brick(name, category, part string) Piece {
	return Piece{Part: part, PartName: name, PartCategory: category, Color: "5"}
}

func TestEditorMatchesTypesSizesPartsColours(t *testing.T) {
	p := mustDoc(t, Document{Name: "Test", Rules: []Rule{
		{ID: "red-2x4", Name: "Red 2 x 4 bricks", Types: []string{"Brick"}, Sizes: []string{"4 x 2"}, Colors: []string{"5"}},
		{ID: "bricks", Name: "Bricks", Types: []string{"Brick"}, Split: SplitSize},
		{ID: "plates", Name: "All plates", Families: []string{"Plate"}},
		{ID: "pins", Name: "Pins", Parts: []string{"2780"}},
	}})
	cases := []struct {
		piece Piece
		want  string
	}{
		{brick("Brick 2 x 4", "Brick", "3001"), "red-2x4"},
		{Piece{Part: "3001", PartName: "Brick 2 x 4", PartCategory: "Brick", Color: "11"}, "bricks/size:2x4"},
		{brick("Brick 1 x 2", "Brick", "3004"), "bricks/size:1x2"},
		{brick("Brick, Round 2 x 2", "Brick, Round", "3941"), Misc}, // only standard bricks
		{brick("Plate, Round 1 x 1", "Plate, Round", "4073"), "plates"},
		{brick("Plate 2 x 2", "Plate", "3022"), "plates"},
		{brick("Technic, Pin", "Technic, Pin", "2780"), "pins"},
		{brick("Technic, Pin", "Technic, Pin", "2780pb01"), "pins"}, // a variant falls back
	}
	for _, c := range cases {
		assert.Equal(t, c.want, p.Route(c.piece, nil), "%+v", c.piece)
	}
	assert.Equal(t, "Bricks 2 x 4", p.CategoryName("bricks/size:2x4"))
	assert.True(t, p.Accepts("bricks/size:1x16"), "a size made as pieces arrive")
	assert.False(t, p.Accepts("plates/size:2x2"), "plates don't split")

	m := p.Match(brick("Plate 2 x 2", "Plate", "3022"), nil)
	assert.Equal(t, Match{Category: "plates", RuleID: "plates", RuleName: "All plates"}, m)
}

func TestEditorDisabledRulesAndLeftovers(t *testing.T) {
	doc := Document{Name: "Test", Leftover: LeftoverPartType, Rules: []Rule{
		{ID: "plates", Name: "Plates", Families: []string{"Plate"}, Disabled: true},
	}}
	p := mustDoc(t, doc)
	assert.Equal(t, "Plate", p.Route(brick("Plate 2 x 2", "Plate", "3022"), nil))
	assert.True(t, p.Accepts("Hinge"))

	doc.Leftover = LeftoverColor
	p = mustDoc(t, doc)
	assert.Equal(t, "color_5", p.Route(brick("Plate 2 x 2", "Plate", "3022"), nil))
	assert.Equal(t, Misc, p.Route(Piece{Part: "3022", PartCategory: "Plate"}, nil), "colour unknown")
	assert.True(t, p.Summary().FallbackColor)

	doc.Leftover = LeftoverMisc
	m := mustDoc(t, doc).Match(brick("Plate 2 x 2", "Plate", "3022"), nil)
	assert.Equal(t, Match{Category: Misc, Leftover: true}, m)
}

func TestEditorSplits(t *testing.T) {
	p := mustDoc(t, Document{Name: "Test", Rules: []Rule{
		{ID: "slopes", Name: "Slopes", Families: []string{"Slope"}, Split: SplitType},
		{ID: "tech", Name: "Technic", Families: []string{"Technic"}, Split: SplitColor},
		{ID: "all", Name: "Everything", Split: SplitFamily},
	}})
	assert.Equal(t, "slopes/type:Slope, Curved", p.Route(brick("Slope, Curved 2 x 1", "Slope, Curved", "11477"), nil))
	assert.Equal(t, "tech/color_5", p.Route(brick("Technic, Liftarm 1 x 5", "Technic, Liftarm", "32316"), nil))
	assert.Equal(t, "all/family:Hinge", p.Route(brick("Hinge Plate 1 x 2", "Hinge", "x"), nil))
	assert.Equal(t, "Slopes · Slope, Curved", p.CategoryName("slopes/type:Slope, Curved"))
	assert.Equal(t, "Technic · Color 5", p.CategoryName("tech/color_5"))
	color, ok := SplitColorOf("tech/color_5")
	assert.True(t, ok)
	assert.Equal(t, "5", color)
}

func TestEditorSetsFillSeparately(t *testing.T) {
	p := mustDoc(t, Document{Name: "Sets", Rules: []Rule{
		{ID: "sets", Name: "My sets", Sets: []RuleSet{
			{Number: "1-1", Name: "First", Parts: []KitLine{{Part: "3001", Color: "5", Need: 2}}},
			{Number: "2-1", Name: "Second", Parts: []KitLine{{Part: "3001", Color: "5", Need: 1}, {Part: "3020", Color: "-1", Need: 1}}},
		}},
		{ID: "bricks", Name: "Bricks", Types: []string{"Brick"}},
	}})
	assert.Equal(t, "1-1 First", p.CategoryName("sets/set:1-1"))
	assert.Len(t, p.Kits(), 2)
	assert.True(t, p.Summary().SetBased)

	full := map[string]bool{}
	kitFull := func(category, part, color string) bool { return full[category] }
	piece := brick("Brick 2 x 4", "Brick", "3001")
	assert.Equal(t, "sets/set:1-1", p.Route(piece, kitFull))
	full["sets/set:1-1"] = true
	assert.Equal(t, "sets/set:2-1", p.Route(piece, kitFull), "the next set takes it")
	full["sets/set:2-1"] = true
	assert.Equal(t, "bricks", p.Route(piece, kitFull), "full kits let it fall through")
	assert.Equal(t, "sets/set:2-1", p.Route(Piece{Part: "3020", Color: "11"}, nil), "an any-colour line")

	cats := []string{}
	for _, c := range p.Summary().Categories {
		cats = append(cats, c.ID)
	}
	assert.Equal(t, []string{"sets/set:1-1", "sets/set:2-1", "bricks"}, cats)
}

func TestNormalize(t *testing.T) {
	_, err := Document{}.Normalize()
	assert.ErrorIs(t, err, ErrInvalid, "a name is needed")

	doc, err := Document{Name: " P ", Rules: []Rule{
		{Name: "", Sizes: []string{"4x2", "2 x 4"}},
		{ID: "dup", Name: "A"}, {ID: "dup", Name: "B"},
		{Name: "Set", Types: []string{"Brick"}, Sets: []RuleSet{{Number: "1-1"}, {Number: "1-1"}}},
	}}.Normalize()
	require.NoError(t, err)
	assert.Equal(t, "P", doc.Name)
	assert.Equal(t, LeftoverMisc, doc.Leftover)
	assert.Equal(t, "Rule 1", doc.Rules[0].Name)
	assert.Equal(t, []string{"2x4"}, doc.Rules[0].Sizes)
	ids := map[string]bool{}
	for _, r := range doc.Rules {
		assert.False(t, ids[r.ID], "ids are unique")
		ids[r.ID] = true
	}
	assert.Nil(t, doc.Rules[3].Types, "a set rule only takes its sets")
	assert.Len(t, doc.Rules[3].Sets, 1)

	_, err = Document{Name: "P", Rules: []Rule{{Sizes: []string{"huge"}}}}.Normalize()
	assert.ErrorIs(t, err, ErrInvalid)
	_, err = Document{Name: "P", Leftover: "bin"}.Normalize()
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestEditorDocumentRoundTrip(t *testing.T) {
	doc, ok := StarterDocument("sizes_then_types")
	require.True(t, ok)
	data, err := DocumentJSON(doc)
	require.NoError(t, err)
	p, err := Parse(data)
	require.NoError(t, err)
	assert.Equal(t, FormatEditor, p.Summary().Format)
	assert.Equal(t, "Bricks 2 x 4", p.CategoryName("rule-1/size:2x4"))
	assert.Equal(t, "rule-2/size:2x2", p.Route(brick("Plate 2 x 2", "Plate", "3022"), nil))
	assert.Equal(t, "Tile", p.Route(brick("Tile 1 x 2", "Tile", "3069b"), nil))
	got, ok := p.Document()
	require.True(t, ok)
	assert.Len(t, got.Rules, 2)
	assert.Equal(t, []RuleInfo{{ID: "rule-1", Name: "Bricks", Description: "Standard bricks, a bin per size."},
		{ID: "rule-2", Name: "Plates", Description: "Standard plates, a bin per size."}}, p.Summary().Rules)
}

func TestPresetsAreValid(t *testing.T) {
	for _, preset := range RulePresets {
		_, err := Document{Name: "P", Rules: []Rule{preset.Rule}}.Normalize()
		assert.NoError(t, err, preset.Key)
	}
	for _, s := range StarterProfiles {
		doc, ok := StarterDocument(s.Key)
		require.True(t, ok)
		assert.Len(t, doc.Rules, len(s.Rules), s.Key)
		_, err := FromDocument(doc)
		assert.NoError(t, err, s.Key)
	}
}

// A Hive profile converted to editor rules routes the same pieces to the same
// categories.
func TestConvertedHiveProgramRoutesTheSame(t *testing.T) {
	hive := mustParse(t, programProfile)
	doc := hive.EditableDocument()
	data, err := DocumentJSON(doc)
	require.NoError(t, err)
	edited, err := Parse(data)
	require.NoError(t, err)
	cases := []struct{ part, color string }{
		{"3003", "5"}, {"3003", "7"}, {"3068b", "12"}, {"unknown-part", "12"}, {"3068b", "7"},
		{"unknown-part", "7"}, {"3003", "any_color"}, {"3001", "5"}, {"3001pb028", "5"},
		{"3004pb038", "7"}, {"3068bpb0001", "7"}, {"3020", "7"},
	}
	for _, c := range cases {
		piece := Piece{Part: c.part, Color: c.color}
		assert.Equal(t, hive.Route(piece, nil), edited.Route(piece, nil), "%s in %q", c.part, c.color)
	}
	assert.Equal(t, hive.CategoryName("red-bricks"), edited.CategoryName("red-bricks"))
}

func TestConvertedPartMapRoutesTheSame(t *testing.T) {
	src := map[string]any{
		"name": "Flat", "default_category_id": "other",
		"categories": map[string]any{"b": map[string]string{"name": "Bricks"}, "rb": map[string]string{"name": "Red bricks"}},
		"part_to_category": map[string]string{
			"any_color-3001": "b", "5-3001": "rb", "any_color-3004": "b",
		},
	}
	data, _ := json.Marshal(src)
	hive := mustParse(t, string(data))
	conv, err := DocumentJSON(hive.EditableDocument())
	require.NoError(t, err)
	edited, err := Parse(conv)
	require.NoError(t, err)
	for _, piece := range []Piece{{Part: "3001", Color: "5"}, {Part: "3001", Color: "7"}, {Part: "3004", Color: "5"}, {Part: "9999", Color: "5"}} {
		assert.Equal(t, hive.Route(piece, nil), edited.Route(piece, nil), "%+v", piece)
	}
}

func TestConvertedBuiltins(t *testing.T) {
	byType := mustParse(t, string(BuiltinData(BuiltinPartType))).EditableDocument()
	assert.Equal(t, LeftoverPartType, byType.Leftover)
	byColor := mustParse(t, string(BuiltinData(BuiltinColor))).EditableDocument()
	assert.Equal(t, LeftoverColor, byColor.Leftover)
}

// A set rule with no sets yet takes nothing: it never becomes a catch-all.
func TestEmptySetRuleTakesNothing(t *testing.T) {
	p := mustDoc(t, Document{Name: "P", Rules: []Rule{{ID: "sets", Name: "Sets", Kind: KindSets}}})
	assert.Equal(t, Misc, p.Route(brick("Brick 2 x 4", "Brick", "3001"), nil))
	doc, _ := p.Document()
	assert.Equal(t, KindSets, doc.Rules[0].Kind)
	_, err := Document{Name: "P", Rules: []Rule{{Kind: "magic"}}}.Normalize()
	assert.ErrorIs(t, err, ErrInvalid)
}
