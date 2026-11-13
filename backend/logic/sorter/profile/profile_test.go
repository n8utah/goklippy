package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cases ported from Sorter V2's tests/test_sorting_profile_program.py.
const programProfile = `{
  "name": "Test", "schema_version": 1,
  "categories": {"red-bricks": {"name": "Red bricks"}, "bricks": {"name": "Bricks"}},
  "program": {
    "format": 1,
    "rules": [
      {"category": "kit", "kit": {"3001": ["5"], "3020": [null]}},
      {"category": "red-bricks", "parts": ["3001", "3003"], "colors": ["5"]},
      {"category": "bricks", "parts": ["3001", "3003", "3004"], "colors": null},
      {"category": "clear", "parts": null, "colors": [12]}
    ],
    "fallback": {"by": "category", "map": {"3020": "bl_26", "3068b": "bl_37"}},
    "default": "misc"
  }
}`

func mustParse(t *testing.T, data string) *Profile {
	t.Helper()
	p, err := Parse([]byte(data))
	require.NoError(t, err)
	return p
}

func TestProgramRouting(t *testing.T) {
	p := mustParse(t, programProfile)
	cases := []struct{ part, color, want string }{
		{"3003", "5", "red-bricks"}, // first rule that takes it
		{"3003", "7", "bricks"},
		{"3068b", "12", "clear"}, // a rule for any part, by color
		{"unknown-part", "12", "clear"},
		{"3068b", "7", "bl_37"},         // fallback map
		{"unknown-part", "7", "misc"},   // default
		{"3003", "any_color", "bricks"}, // unknown color: only any-color rules
		{"3003", "", "bricks"},
		{"3001", "5", "kit"},             // without a tracker a kit keeps taking
		{"3001pb028", "5", "red-bricks"}, // a variant takes its base part...
		{"3004pb038", "7", "bricks"},
		{"3068bpb0001", "7", "bl_37"},
		{"3020", "7", "kit"}, // any-color kit line
	}
	for _, c := range cases {
		assert.Equal(t, c.want, p.Route(Piece{Part: c.part, Color: c.color}, nil), "%s in %q", c.part, c.color)
	}
}

func TestProgramVariantNeverFillsAKit(t *testing.T) {
	p := mustParse(t, programProfile)
	// 3001pb028 in red is not the exact kit part, so it skips the kit.
	assert.Equal(t, "red-bricks", p.Route(Piece{Part: "3001pb028", Color: "5"}, nil))
}

func TestKitPassesPiecesOnOnceFull(t *testing.T) {
	p := mustParse(t, programProfile)
	full := func(category, part, color string) bool {
		return category == "kit" && part == "3001" && color == "5"
	}
	assert.Equal(t, "red-bricks", p.Route(Piece{Part: "3001", Color: "5"}, full))
	assert.Equal(t, "kit", p.Route(Piece{Part: "3020", Color: "7"}, full), "other kit lines still collect")
}

func TestColorFallback(t *testing.T) {
	p := mustParse(t, `{"name": "C", "program": {"rules": [], "fallback": {"by": "color"}}}`)
	assert.Equal(t, "color_7", p.Route(Piece{Part: "3068b", Color: "7"}, nil))
	assert.Equal(t, "misc", p.Route(Piece{Part: "3068b", Color: ""}, nil))
	assert.True(t, p.Summary().FallbackColor)
	assert.Equal(t, "Color 7", p.CategoryName("color_7"))
}

func TestFlatMap(t *testing.T) {
	p := mustParse(t, `{"name": "Flat", "part_to_category": {"5-3001": "red", "any_color-3001": "bricks"}, "default_category_id": "misc"}`)
	assert.Equal(t, "red", p.Route(Piece{Part: "3001", Color: "5"}, nil))
	assert.Equal(t, "bricks", p.Route(Piece{Part: "3001", Color: "7"}, nil))
	assert.Equal(t, "misc", p.Route(Piece{Part: "3002", Color: "7"}, nil))
	assert.Equal(t, "part_map", p.Summary().Format)
}

func TestFlatMapVariantsTakeTheirBasePart(t *testing.T) {
	p := mustParse(t, `{"name": "Flat", "part_to_category": {"5-3010": "red", "any_color-3010": "bricks", "any_color-3464": "wheels"}, "default_category_id": "everything_else"}`)
	assert.Equal(t, "red", p.Route(Piece{Part: "3010pb176R", Color: "5"}, nil))
	assert.Equal(t, "bricks", p.Route(Piece{Part: "3010pb176R", Color: "7"}, nil))
	assert.Equal(t, "wheels", p.Route(Piece{Part: "3464c02", Color: "11"}, nil))
	assert.Equal(t, "everything_else", p.Route(Piece{Part: "BA002pb01", Color: "11"}, nil))
}

func TestBasePartIDs(t *testing.T) {
	assert.Equal(t, []string{"3010"}, BasePartIDs("3010pb176R"))
	assert.Equal(t, []string{"122c01", "122"}, BasePartIDs("122c01assy2"))
	assert.Equal(t, []string{"50944pb01", "50944"}, BasePartIDs("50944pb01c01"))
	assert.Equal(t, []string{"6014b", "6014"}, BasePartIDs("6014bc01"))
	assert.Empty(t, BasePartIDs("3001"))
}

func TestSummaryAndCategories(t *testing.T) {
	p := mustParse(t, programProfile)
	s := p.Summary()
	assert.Equal(t, "Test", s.Name)
	assert.Equal(t, "program", s.Format)
	assert.Equal(t, "misc", s.DefaultID)
	assert.Equal(t, 5, s.PartCount) // 3001 3003 3004 3020 3068b, each counted once
	ids := make([]string, len(s.Categories))
	for i, c := range s.Categories {
		ids[i] = c.ID
	}
	// Rule order first, then the fallback categories; misc has no bin of its own.
	assert.Equal(t, []string{"kit", "red-bricks", "bricks", "clear", "bl_26", "bl_37"}, ids)
	assert.Equal(t, "Red bricks", s.Categories[1].Name)
}

func TestParseRejectsBadProfiles(t *testing.T) {
	for name, data := range map[string]string{
		"not json":      `{`,
		"no map":        `{"name": "x"}`,
		"wrong version": `{"name": "x", "schema_version": 2, "part_to_category": {}}`,
		"rule category": `{"name": "x", "program": {"rules": [{"parts": ["1"]}]}}`,
	} {
		_, err := Parse([]byte(data))
		assert.ErrorIs(t, err, ErrInvalid, name)
	}
}
