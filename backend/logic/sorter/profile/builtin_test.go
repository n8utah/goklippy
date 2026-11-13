package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinPartType(t *testing.T) {
	p, err := Parse(BuiltinData(BuiltinPartType))
	require.NoError(t, err)
	s := p.Summary()
	assert.Equal(t, "By part type", s.Name)
	assert.Equal(t, FormatBuiltin, s.Format)
	assert.Equal(t, BuiltinPartType, s.Builtin)
	assert.Equal(t, Category{ID: "Brick", Name: "Brick"}, s.Categories[0], "most common first, for pre-assigning")

	cases := []struct{ category, want string }{
		{"Plate", "Plate"},
		{"Plate, Round", "Plate"},
		{"Plate, Modified", "Plate"},
		{"Technic, Liftarm", "Technic"},
		{"Slope, Curved, Inverted", "Slope"},
		{"Wheel & Tire Assembly", "Wheel & Tire Assembly"},
		{"Bionicle", "Bionicle"}, // not on the list: still its own category
		{"  Tile , Modified", "Tile"},
		{"", Misc},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, p.Route(Piece{Part: "3001", Color: "5", PartCategory: c.category}, nil), c.category)
	}

	assert.True(t, p.Accepts("Plate"))
	assert.True(t, p.Accepts("Bionicle"), "any part type can be given a bin")
	assert.True(t, p.Accepts(Misc))
	assert.False(t, p.Accepts(""))
}

func TestBuiltinColor(t *testing.T) {
	p, err := Parse(BuiltinData(BuiltinColor))
	require.NoError(t, err)
	assert.Equal(t, "By colour", p.Summary().Name)
	assert.Empty(t, p.Summary().Categories, "colours are listed as they arrive")

	assert.Equal(t, "color_5", p.Route(Piece{Part: "3001", Color: "5", PartCategory: "Brick"}, nil))
	assert.Equal(t, Misc, p.Route(Piece{Part: "3001", Color: ""}, nil))
	assert.Equal(t, Misc, p.Route(Piece{Part: "3001", Color: AnyColor}, nil))

	assert.True(t, p.Accepts("color_11"))
	assert.False(t, p.Accepts("Plate"))
}

func TestBuiltinUnknownKind(t *testing.T) {
	_, err := Parse([]byte(`{"builtin": "by_mood"}`))
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestPartFamily(t *testing.T) {
	assert.Equal(t, "Brick", PartFamily("Brick, Round"))
	assert.Equal(t, "", PartFamily(""))
}
