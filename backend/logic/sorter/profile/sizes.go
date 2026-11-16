package profile

import (
	"regexp"
	"strconv"
	"strings"
)

// A size in a part's name: "Brick 2 x 4", "Plate, Modified 2 x 2 x 2/3 with ...".
var sizePattern = regexp.MustCompile(`(\d+(?:/\d+)?)\s*x\s*(\d+(?:/\d+)?)(?:\s*x\s*(\d+(?:/\d+)?))?`)

// PartSize is the size in a part's name as a key, the two footprint sides smallest first
// and any height after: "Brick 4 x 2" -> "2x4", "Brick 1 x 2 x 3" -> "1x2x3". "" when
// the name has no size.
func PartSize(name string) string {
	m := sizePattern.FindStringSubmatch(name)
	if m == nil {
		return ""
	}
	return NormalizeSize(strings.Join(nonEmpty(m[1:]), "x"))
}

// NormalizeSize turns a typed size ("4 x 2", "2X4", "1x2x3") into its key; "" when it
// isn't a size.
func NormalizeSize(size string) string {
	parts := strings.Split(strings.ReplaceAll(strings.ToLower(size), " ", ""), "x")
	if len(parts) < 2 || len(parts) > 3 {
		return ""
	}
	for _, p := range parts {
		if sizeValue(p) <= 0 {
			return ""
		}
	}
	if sizeValue(parts[1]) < sizeValue(parts[0]) {
		parts[0], parts[1] = parts[1], parts[0]
	}
	return strings.Join(parts, "x")
}

// SizeLabel is how a size key reads: "2x4" -> "2 x 4".
func SizeLabel(key string) string {
	return strings.ReplaceAll(key, "x", " x ")
}

func sizeValue(s string) float64 {
	if num, den, ok := strings.Cut(s, "/"); ok {
		n, err1 := strconv.Atoi(num)
		d, err2 := strconv.Atoi(den)
		if err1 != nil || err2 != nil || d == 0 {
			return 0
		}
		return float64(n) / float64(d)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return float64(n)
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// CommonSizes are the usual brick and plate sizes, smallest first: what the editor offers
// before the catalog has seen anything, and the bins a size split pre-assigns.
var CommonSizes = []string{
	"1x1", "1x2", "1x3", "1x4", "1x6", "1x8", "1x10", "1x12", "1x16",
	"2x2", "2x3", "2x4", "2x6", "2x8", "2x10", "2x12", "2x16",
	"3x3", "4x4", "4x6", "4x8", "4x10", "4x12", "6x6", "6x8", "6x10", "6x12", "8x8", "8x16",
	"1x1x3", "1x2x2", "1x2x3", "1x2x5", "1x4x3", "1x6x5",
}
