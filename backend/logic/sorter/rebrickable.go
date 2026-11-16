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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"goklippy/backend/logic/sorter/profile"
)

// A set rule's parts come from Rebrickable's API, once per set, when the set is added to
// the rule. Rebrickable has its own part and colour ids; each part and colour carries
// its BrickLink equivalents (external_ids), which is what Brickognize reports.

var (
	ErrRebrickableNoKey = errors.New("no Rebrickable API key yet: add one on System › Settings › Services")
	ErrSetNotFound      = errors.New("Rebrickable doesn't know that set number")
)

// RebrickableStore keeps the sorter's Rebrickable API key.
type RebrickableStore interface {
	RebrickableKey(ctx context.Context) (string, error)
	SaveRebrickableKey(ctx context.Context, key string) error
}

// Rebrickable looks sets up.
type Rebrickable struct {
	store   RebrickableStore
	client  *http.Client
	baseURL string
}

const rebrickableURL = "https://rebrickable.com/api/v3"

func NewRebrickable(store RebrickableStore) *Rebrickable {
	return &Rebrickable{store: store, client: &http.Client{Timeout: 30 * time.Second}, baseURL: rebrickableURL}
}

// WithBaseURL points it elsewhere (tests).
func (r *Rebrickable) WithBaseURL(u string) *Rebrickable {
	r.baseURL = strings.TrimRight(u, "/")
	return r
}

// KeySet reports whether a key has been saved.
func (r *Rebrickable) KeySet(ctx context.Context) (bool, error) {
	key, err := r.store.RebrickableKey(ctx)
	return key != "", err
}

// SaveKey saves the key; an empty key removes it.
func (r *Rebrickable) SaveKey(ctx context.Context, key string) error {
	return r.store.SaveRebrickableKey(ctx, strings.TrimSpace(key))
}

var setNumber = regexp.MustCompile(`^[0-9A-Za-z]+(-[0-9]+)?$`)

// NormalizeSetNumber adds the "-1" a set number usually has: "75192" -> "75192-1".
func NormalizeSetNumber(n string) (string, error) {
	n = strings.TrimSpace(n)
	if !setNumber.MatchString(n) {
		return "", fmt.Errorf("%w: %q isn't a set number (like 75192 or 75192-1)", profile.ErrInvalid, n)
	}
	if !strings.Contains(n, "-") {
		n += "-1"
	}
	return n, nil
}

type rbSet struct {
	SetNum   string `json:"set_num"`
	Name     string `json:"name"`
	Year     int    `json:"year"`
	NumParts int    `json:"num_parts"`
	ImageURL string `json:"set_img_url"`
}

type rbExternal struct {
	BrickLink json.RawMessage `json:"BrickLink"`
}

type rbSetPart struct {
	Part struct {
		PartNum     string     `json:"part_num"`
		Name        string     `json:"name"`
		ExternalIDs rbExternal `json:"external_ids"`
	} `json:"part"`
	Color struct {
		ID          int        `json:"id"`
		Name        string     `json:"name"`
		ExternalIDs rbExternal `json:"external_ids"`
	} `json:"color"`
	Quantity int  `json:"quantity"`
	IsSpare  bool `json:"is_spare"`
}

// LookupSet fetches a set and its parts as BrickLink ids, without spare parts or
// minifigures. Parts Rebrickable gives no BrickLink number for keep Rebrickable's (they
// are usually the same); colours it can't map are listed in Unmapped and the line takes
// any colour.
func (r *Rebrickable) LookupSet(ctx context.Context, number string) (profile.RuleSet, error) {
	number, err := NormalizeSetNumber(number)
	if err != nil {
		return profile.RuleSet{}, err
	}
	key, err := r.store.RebrickableKey(ctx)
	if err != nil {
		return profile.RuleSet{}, err
	}
	if key == "" {
		return profile.RuleSet{}, ErrRebrickableNoKey
	}
	var set rbSet
	if err := r.get(ctx, key, r.baseURL+"/lego/sets/"+url.PathEscape(number)+"/", &set); err != nil {
		return profile.RuleSet{}, err
	}
	out := profile.RuleSet{Number: set.SetNum, Name: set.Name, Year: set.Year, ImageURL: set.ImageURL}
	merged := map[[2]string]int{}
	unmapped := map[string]bool{}
	next := r.baseURL + "/lego/sets/" + url.PathEscape(number) + "/parts/?page_size=1000&inc_part_details=1"
	for next != "" {
		var page struct {
			Next    *string     `json:"next"`
			Results []rbSetPart `json:"results"`
		}
		if err := r.get(ctx, key, next, &page); err != nil {
			return profile.RuleSet{}, err
		}
		for _, p := range page.Results {
			if p.IsSpare || p.Quantity <= 0 {
				continue
			}
			part := firstBrickLinkPart(p.Part.ExternalIDs.BrickLink)
			if part == "" {
				part = p.Part.PartNum
			}
			color := ""
			if p.Color.ID >= 0 { // -1 is Rebrickable's "unknown colour"
				if color = firstBrickLinkColor(p.Color.ExternalIDs.BrickLink); color == "" {
					unmapped[fmt.Sprintf("%s in %s (no BrickLink colour; any colour counts)", part, p.Color.Name)] = true
				}
			}
			merged[[2]string{part, color}] += p.Quantity
		}
		next = ""
		if page.Next != nil {
			next = *page.Next
		}
	}
	for k, need := range merged {
		out.Parts = append(out.Parts, profile.KitLine{Part: k[0], Color: k[1], Need: need})
	}
	sort.Slice(out.Parts, func(i, j int) bool {
		if out.Parts[i].Part != out.Parts[j].Part {
			return out.Parts[i].Part < out.Parts[j].Part
		}
		return out.Parts[i].Color < out.Parts[j].Color
	})
	if out.Parts == nil {
		out.Parts = []profile.KitLine{}
	}
	for u := range unmapped {
		out.Unmapped = append(out.Unmapped, u)
	}
	sort.Strings(out.Unmapped)
	slog.Info("set looked up on Rebrickable", "set", out.Number, "lines", len(out.Parts), "unmapped", len(out.Unmapped))
	return out, nil
}

func (r *Rebrickable) get(ctx context.Context, key, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "key "+key)
	req.Header.Set("Accept", "application/json")
	res, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("reach Rebrickable: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	switch {
	case res.StatusCode == http.StatusNotFound:
		return ErrSetNotFound
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w: Rebrickable refused the API key; check it on System › Settings › Services", ErrInvalidSettings)
	case res.StatusCode != http.StatusOK:
		return fmt.Errorf("Rebrickable answered %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("read Rebrickable's answer: %w", err)
	}
	return nil
}

// A part's BrickLink ids are a list of strings: ["3001"].
func firstBrickLinkPart(raw json.RawMessage) string {
	var ids []string
	if json.Unmarshal(raw, &ids) == nil && len(ids) > 0 {
		return ids[0]
	}
	return ""
}

// A colour's are {"ext_ids": [11], "ext_descrs": [["Black"]]}.
func firstBrickLinkColor(raw json.RawMessage) string {
	var c struct {
		ExtIDs []int `json:"ext_ids"`
	}
	if json.Unmarshal(raw, &c) == nil && len(c.ExtIDs) > 0 {
		return strconv.Itoa(c.ExtIDs[0])
	}
	return ""
}
