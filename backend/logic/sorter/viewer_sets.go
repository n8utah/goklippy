package sorter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"unicode"

	"goklippy/backend/logic/sorter/profile"
)

// A set rule's parts come from the sort viewer site first: it keeps every set from
// Rebrickable's download files, so no API key is needed. Its part numbers are
// Rebrickable's (usually BrickLink's too) and its colours are names, mapped here to the
// sorter's BrickLink colour ids by name.

// ErrViewerSetsNotReady is the site still downloading its set data.
var ErrViewerSetsNotReady = errors.New("the viewer site's set data isn't ready yet; try again in a few minutes")

// ErrViewerTooOld is a site from before set lookups.
var ErrViewerTooOld = errors.New("the viewer site doesn't look sets up yet: deploy the newer sort viewer")

type viewerSet struct {
	SetNum string `json:"set_num"`
	Name   string `json:"name"`
	Year   int    `json:"year"`
	ImgURL string `json:"img_url"`
	Parts  []struct {
		PartNum   string `json:"part_num"`
		ColorName string `json:"color_name"`
		Need      int    `json:"need"`
	} `json:"parts"`
}

// LookupSet fetches a set's parts from the viewer site; number is a full set number
// ("75192-1"). Colours the sorter has no BrickLink id for are listed in Unmapped and
// the line takes any colour.
func (s *Sharing) LookupSet(ctx context.Context, number string) (profile.RuleSet, error) {
	set, err := s.store.SharingSettings(ctx)
	if err != nil {
		return profile.RuleSet{}, err
	}
	if set.URL == "" || set.APIKey == "" {
		return profile.RuleSet{}, ErrSharingNotSetUp
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, set.URL+"/api/push/sets/"+url.PathEscape(number), nil)
	if err != nil {
		return profile.RuleSet{}, err
	}
	req.Header.Set("Authorization", "Bearer "+set.APIKey)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return profile.RuleSet{}, fmt.Errorf("can't reach the viewer site: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		// A site too old to look sets up answers "no such endpoint" (or a bare 404).
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &e) != nil || e.Error == "" || e.Error == "no such endpoint" {
			return profile.RuleSet{}, ErrViewerTooOld
		}
		return profile.RuleSet{}, ErrSetNotFound
	case http.StatusServiceUnavailable:
		return profile.RuleSet{}, ErrViewerSetsNotReady
	case http.StatusUnauthorized:
		return profile.RuleSet{}, errors.New("the viewer site refused the key: make a new one under its Admin → Sorter keys")
	default:
		return profile.RuleSet{}, fmt.Errorf("the viewer site answered %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var vs viewerSet
	if err := json.Unmarshal(body, &vs); err != nil {
		return profile.RuleSet{}, fmt.Errorf("read the viewer site's answer: %w", err)
	}

	colors, err := s.colorIDsByName(ctx)
	if err != nil {
		return profile.RuleSet{}, err
	}
	out := profile.RuleSet{Number: vs.SetNum, Name: vs.Name, Year: vs.Year, ImageURL: vs.ImgURL, Source: "viewer"}
	merged := map[[2]string]int{}
	unmapped := map[string]bool{}
	for _, p := range vs.Parts {
		if p.Need <= 0 || p.PartNum == "" {
			continue
		}
		color := ""
		if key := colorKey(p.ColorName); key != "" && !anyColor[key] {
			if color = colors[key]; color == "" {
				unmapped[fmt.Sprintf("%s in %s (no BrickLink colour; any colour counts)", p.PartNum, p.ColorName)] = true
			}
		}
		merged[[2]string{p.PartNum, color}] += p.Need
	}
	out.Parts = kitLines(merged)
	for u := range unmapped {
		out.Unmapped = append(out.Unmapped, u)
	}
	sort.Strings(out.Unmapped)
	slog.Info("set looked up on the viewer site", "set", out.Number, "lines", len(out.Parts), "unmapped", len(out.Unmapped))
	return out, nil
}

// anyColor are Rebrickable's colours that mean "no particular colour".
var anyColor = map[string]bool{"unknown": true, "nocoloranycolor": true, "anycolor": true}

// colorIDsByName maps folded colour names to BrickLink colour ids: the usual colours,
// then every colour the sorter has identified.
func (s *Sharing) colorIDsByName(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	for id, name := range profile.CommonColors {
		out[colorKey(name)] = id
	}
	if s.colorNames != nil {
		names, err := s.colorNames(ctx)
		if err != nil {
			return nil, err
		}
		for id, name := range names {
			if name != "" {
				out[colorKey(name)] = id
			}
		}
	}
	return out, nil
}

// colorKey folds a colour name for matching across BrickLink and Rebrickable, as the
// viewer site does: "Light Bluish Gray" and "light-bluish gray" both give
// "lightbluishgray".
func colorKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// kitLines turns merged (part, colour) counts into sorted kit lines.
func kitLines(merged map[[2]string]int) []profile.KitLine {
	out := make([]profile.KitLine, 0, len(merged))
	for k, need := range merged {
		out = append(out, profile.KitLine{Part: k[0], Color: k[1], Need: need})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Part != out[j].Part {
			return out[i].Part < out[j].Part
		}
		return out[i].Color < out[j].Color
	})
	return out
}
