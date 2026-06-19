package server

import (
	"net/url"
	"sort"
	"strconv"
	"strings"

	"note-aura/internal/db"
)

// ----- sorting -----

// noteSortOptions are the user-selectable orderings for the notes list.
var noteSortOptions = []struct{ Key, Label string }{
	{"modified", "Modified date"},
	{"created", "Created date"},
	{"title", "Title"},
	{"category", "Category"},
	{"tag", "Tag"},
	{"source", "Added method"},
}

func validSort(key string) string {
	for _, o := range noteSortOptions {
		if o.Key == key {
			return key
		}
	}
	return "modified"
}

// sortNotes orders notes in place by the chosen key (stable).
func sortNotes(notes []*db.Note, key string) {
	switch key {
	case "created":
		sort.SliceStable(notes, func(i, j int) bool { return notes[i].CreatedAt.After(notes[j].CreatedAt) })
	case "title":
		sort.SliceStable(notes, func(i, j int) bool {
			return lessBlankLast(strings.ToLower(notes[i].Title), strings.ToLower(notes[j].Title))
		})
	case "category":
		sort.SliceStable(notes, func(i, j int) bool {
			return lessBlankLast(strings.ToLower(notes[i].CategoryName), strings.ToLower(notes[j].CategoryName))
		})
	case "tag":
		sort.SliceStable(notes, func(i, j int) bool { return lessBlankLast(firstTag(notes[i]), firstTag(notes[j])) })
	case "source":
		sort.SliceStable(notes, func(i, j int) bool {
			if notes[i].SourceType != notes[j].SourceType {
				return notes[i].SourceType < notes[j].SourceType
			}
			return notes[i].UpdatedAt.After(notes[j].UpdatedAt)
		})
	default: // modified
		sort.SliceStable(notes, func(i, j int) bool { return notes[i].UpdatedAt.After(notes[j].UpdatedAt) })
	}
}

func firstTag(n *db.Note) string {
	if len(n.Tags) > 0 {
		return strings.ToLower(n.Tags[0])
	}
	return ""
}

// lessBlankLast compares strings ascending, but sorts empty values to the end.
func lessBlankLast(a, b string) bool {
	switch {
	case a == b:
		return false
	case a == "":
		return false
	case b == "":
		return true
	default:
		return a < b
	}
}

// ----- pagination -----

var perPageOptions = []int{10, 25, 50, 100}

func validPerPage(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	for _, o := range perPageOptions {
		if n == o {
			return n
		}
	}
	return 10
}

// notesURL builds a /notes link preserving the current view parameters.
func notesURL(q, category, tag, sortKey string, per, page int) string {
	v := url.Values{}
	if q != "" {
		v.Set("q", q)
	}
	if category != "" {
		v.Set("category", category)
	}
	if tag != "" {
		v.Set("tag", tag)
	}
	if sortKey != "" && sortKey != "modified" {
		v.Set("sort", sortKey)
	}
	if per != 0 && per != 10 {
		v.Set("per", strconv.Itoa(per))
	}
	if page > 1 {
		v.Set("page", strconv.Itoa(page))
	}
	if enc := v.Encode(); enc != "" {
		return "/notes?" + enc
	}
	return "/notes"
}

// ----- categories (hierarchical via "Parent/Child" paths) -----

// categoryMatches reports whether a note's category falls under the filter — the
// exact category or any of its sub-categories.
func categoryMatches(noteCat, filter string) bool {
	if strings.EqualFold(noteCat, filter) {
		return true
	}
	return strings.HasPrefix(strings.ToLower(noteCat), strings.ToLower(filter)+"/")
}

// catRow is one node of the category tree for the sidebar.
type catRow struct {
	Path  string // full "Parent/Child" path (used for the filter link)
	Leaf  string // last segment (shown, indented)
	Depth int    // number of ancestors (indent level)
	Count int    // notes directly in this category
}

// buildCatTree expands the user's categories (and their ancestors) into an
// indented, path-sorted tree for the sidebar.
func (s *Server) buildCatTree(userID int64) []catRow {
	cats, _ := s.db.CategoriesWithCounts(userID)
	own := map[string]int{}
	for _, c := range cats {
		own[c.Name] += c.Count
	}
	set := map[string]bool{}
	for p := range own {
		parts := strings.Split(p, "/")
		for i := 1; i <= len(parts); i++ {
			set[strings.Join(parts[:i], "/")] = true
		}
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	rows := make([]catRow, 0, len(paths))
	for _, p := range paths {
		parts := strings.Split(p, "/")
		rows = append(rows, catRow{Path: p, Leaf: parts[len(parts)-1], Depth: len(parts) - 1, Count: own[p]})
	}
	return rows
}
