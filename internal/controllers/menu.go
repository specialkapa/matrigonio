package controllers

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/specialkapa/matrigonio/internal/server"
	"github.com/starfederation/datastar-go/datastar"
)

type MenuController struct {
	*server.APIConfig
}

type menuLookupRequest struct {
	GuestName string `json:"guestName"`
	// SuggestIndex is which fuzzy suggestion to offer. It is bumped each time
	// the guest rejects a "did you mean?" guess, walking down the ranked list.
	SuggestIndex int `json:"suggestIndex"`
	// Confirm is set when the guest accepts the suggestion at SuggestIndex, so
	// we reveal that guest's menu directly (no need to round-trip the name
	// through the bound input signal).
	Confirm bool `json:"confirm"`
}

func (c *MenuController) HandlerMenuLookup(w http.ResponseWriter, r *http.Request) {
	// ReadSignals pulls Datastar's signals regardless of transport (JSON body
	// for @post, datastar= query param for @get), so this isn't coupled to the
	// request method the way a manual body decode would be.
	var req menuLookupRequest
	_ = datastar.ReadSignals(r, &req)
	query := strings.TrimSpace(req.GuestName)

	var inner string
	switch {
	case query == "":
		inner = `<p class="menu-result-msg">Type your first name above to see your choices.</p>`
	case c.Guests == nil:
		inner = `<p class="menu-result-msg">The guest list isn't available right now. Please try again later.</p>`
	default:
		guest, suggestions, found := c.Guests.Lookup(query)
		switch {
		case found:
			inner = renderGuest(guest)
		case req.Confirm && req.SuggestIndex >= 0 && req.SuggestIndex < len(suggestions):
			// Guest accepted the "did you mean?" guess at this index.
			inner = renderGuest(suggestions[req.SuggestIndex])
		case len(suggestions) > 0:
			inner = renderSuggestion(query, suggestions, req.SuggestIndex)
		default:
			inner = renderNotFound(query)
		}
	}

	// useViewTransition lets the browser cross-fade/slide the result as it
	// changes (e.g. cycling through "did you mean?" guesses). Datastar falls back
	// to an instant swap where View Transitions aren't supported.
	sse := datastar.NewSSE(w, r)
	_ = sse.PatchElements(
		`<div id="menu-result" class="menu-result">`+inner+`</div>`,
		datastar.WithUseViewTransitions(true),
	)
}

func renderGuest(g server.Guest) string {
	name := html.EscapeString(g.Name)
	var b strings.Builder

	if !g.HasChoices() {
		fmt.Fprintf(&b, `<p class="menu-result-msg">We've got you on the list, %s, but we don't have your menu choices recorded yet. If that doesn't sound right, just give us a shout.</p>`, name)
		writeDietary(&b, g)
		return b.String()
	}

	fmt.Fprintf(&b, `<p class="menu-result-name">Hi %s! See what you'll be having below.</p><ul class="info-list">`, name)
	b.WriteString(courseRow("starter", g.Starter))
	b.WriteString(courseRow("intermediate", g.Intermediate))
	b.WriteString(courseRow("main", g.Main))
	b.WriteString(courseRow("dessert", g.Dessert))
	b.WriteString(`</ul>`)
	writeDietary(&b, g)
	return b.String()
}

func writeDietary(b *strings.Builder, g server.Guest) {
	if g.Allergies != "" {
		fmt.Fprintf(b, `<p class="menu-result-note">noted dietary: %s</p>`, html.EscapeString(g.Allergies))
	}
}

func courseRow(label, value string) string {
	if value == "" {
		value = "—"
	}
	return fmt.Sprintf(`<li><strong>%s</strong>: %s</li>`, label, html.EscapeString(value))
}

// renderSuggestion offers one ranked fuzzy match (at idx) as a yes/no
// confirmation. "yes" re-runs the lookup with the exact name (which then
// matches and reveals the menu); "no" bumps the index so the next-best guess is
// offered on the following request. Once the list is exhausted it gives up.
func renderSuggestion(query string, suggestions []server.Guest, idx int) string {
	if idx < 0 {
		idx = 0
	}
	if idx >= len(suggestions) {
		return `<p class="menu-result-msg">That's all the close matches we have. Double-check the spelling, or get in touch and we'll sort it out.</p>`
	}
	best := suggestions[idx]

	// The buttons carry plain data-* attributes; clicks are handled by the
	// static delegation handler on .menu-result-wrap (see index.html). "confirm"
	// accepts the suggestion at the current index; "reject" advances to the next
	// one (idx+1). Neither touches the bound guestName signal.
	var b strings.Builder
	fmt.Fprintf(&b, `<p class="menu-result-msg">We couldn't find &ldquo;%s&rdquo;. Did you mean <strong>%s</strong>?</p>`,
		html.EscapeString(query), html.EscapeString(best.Name))
	b.WriteString(`<div class="lookup-suggestions">`)
	b.WriteString(`<button type="button" class="lookup-suggestion" data-action="confirm">yes</button>`)
	fmt.Fprintf(&b, `<button type="button" class="lookup-suggestion lookup-suggestion-no" data-action="reject" data-next="%d">no</button>`,
		idx+1)
	b.WriteString(`</div>`)
	return b.String()
}

func renderNotFound(query string) string {
	return fmt.Sprintf(`<p class="menu-result-msg">We couldn't find &ldquo;%s&rdquo;. Double-check the spelling, or get in touch and we'll sort it out.</p>`, html.EscapeString(query))
}
