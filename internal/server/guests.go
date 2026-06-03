package server

import (
	"encoding/csv"
	"os"
	"sort"
	"strings"
)

// Guest is a single row from the menu-choice CSV.
type Guest struct {
	Name         string
	Allergies    string
	Starter      string
	Intermediate string
	Main         string
	Dessert      string
}

// HasChoices reports whether the guest has any course recorded yet.
func (g Guest) HasChoices() bool {
	return g.Starter != "" || g.Intermediate != "" || g.Main != "" || g.Dessert != ""
}

// GuestStore is an in-memory lookup of guests keyed by normalized name.
type GuestStore struct {
	byName map[string]Guest
}

// CSV column indexes.
const (
	colName         = 0
	colAllergies    = 3
	colStarter      = 4
	colIntermediate = 5
	colMain         = 6
	colDessert      = 7
	minColumns      = 8
)

// LoadGuests reads the menu-choice CSV at path into a GuestStore. The header
// row, the trailing totals row, and any row without a name are skipped.
func LoadGuests(path string) (*GuestStore, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // rows may have a ragged trailing column
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	store := &GuestStore{byName: make(map[string]Guest)}
	for i, rec := range records {
		if i == 0 || len(rec) < minColumns {
			continue // header or malformed row
		}
		name := strings.TrimSpace(rec[colName])
		if name == "" {
			continue // totals row / blank line
		}
		store.byName[normalizeName(name)] = Guest{
			Name:         name,
			Allergies:    strings.TrimSpace(rec[colAllergies]),
			Starter:      strings.TrimSpace(rec[colStarter]),
			Intermediate: strings.TrimSpace(rec[colIntermediate]),
			Main:         strings.TrimSpace(rec[colMain]),
			Dessert:      strings.TrimSpace(rec[colDessert]),
		}
	}
	return store, nil
}

// Lookup resolves a typed query to a guest. It returns an exact match when the
// normalized name is found; otherwise it returns up to five fuzzy suggestions
// (prefix matches and near-spellings) so the caller can offer "did you mean?".
func (gs *GuestStore) Lookup(query string) (guest Guest, suggestions []Guest, found bool) {
	nq := normalizeName(query)
	if nq == "" {
		return Guest{}, nil, false
	}
	if g, ok := gs.byName[nq]; ok {
		return g, nil, true
	}

	type candidate struct {
		guest  Guest
		norm   string
		dist   int
		prefix bool
	}
	var cands []candidate
	limit := maxEditDistance(len(nq))
	for norm, g := range gs.byName {
		switch {
		case len(nq) >= 2 && strings.HasPrefix(norm, nq):
			cands = append(cands, candidate{g, norm, 0, true})
		default:
			if d := levenshtein(nq, norm); d <= limit {
				cands = append(cands, candidate{g, norm, d, false})
			}
		}
	}

	sort.Slice(cands, func(i, j int) bool {
		if cands[i].prefix != cands[j].prefix {
			return cands[i].prefix // prefix matches first
		}
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist // then closest spelling
		}
		return cands[i].norm < cands[j].norm // then alphabetical, for stability
	})
	if len(cands) > 5 {
		cands = cands[:5]
	}

	out := make([]Guest, len(cands))
	for i, c := range cands {
		out[i] = c.guest
	}
	return Guest{}, out, false // found is reserved for exact matches
}

// accentReplacer folds common Latin accented characters down to plain ASCII so
// that, e.g., "José" entered as "Jose" still matches.
var accentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ä", "a", "ã", "a", "å", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "ö", "o", "õ", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ñ", "n", "ç", "c",
)

// normalizeName lowercases, strips accents, and collapses whitespace.
func normalizeName(s string) string {
	s = accentReplacer.Replace(strings.ToLower(strings.TrimSpace(s)))
	return strings.Join(strings.Fields(s), " ")
}

// maxEditDistance scales the allowed number of typos with the query length so
// short names don't over-match.
func maxEditDistance(n int) int {
	switch {
	case n <= 3:
		return 1
	case n <= 6:
		return 2
	default:
		return 3
	}
}

// levenshtein returns the edit distance between two strings.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr := make([]int, len(rb)+1)
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev = curr
	}
	return prev[len(rb)]
}
