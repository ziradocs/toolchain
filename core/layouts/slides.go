// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package layouts

import (
	"sort"

	"go.ziradocs.com/core/v2/ast"
)

// SlideType is the shared declaration of a syntactically recognized slide
// type. Canonical resolves aliases; OpensSection declares the structural
// meaning used by consumers that build agendas or section navigation.
type SlideType struct {
	Canonical    string
	Title        bool
	Content      bool
	OpensSection bool
}

var slideTypes = map[string]SlideType{
	"title":            {Canonical: "title", Title: true},
	"title_slide":      {Canonical: "title", Title: true},
	"cover":            {Canonical: "title", Title: true},
	"intro":            {Canonical: "title", Title: true},
	"content":          {Canonical: "content", Content: true},
	"section":          {Canonical: "section", Content: true, OpensSection: true},
	"chapter":          {Canonical: "section", Content: true, OpensSection: true},
	"code_example":     {Canonical: "code_example", Content: true},
	"with_directive":   {Canonical: "with_directive", Content: true},
	"comparison":       {Canonical: "comparison"},
	"stats":            {Canonical: "stats"},
	"hero":             {Canonical: "hero"},
	"testimonial":      {Canonical: "testimonial"},
	"timeline":         {Canonical: "timeline"},
	"before_after":     {Canonical: "before_after"},
	"pricing":          {Canonical: "pricing"},
	"team":             {Canonical: "team"},
	"feature_showcase": {Canonical: "feature_showcase"},
	"call_to_action":   {Canonical: "call_to_action"},
	"dashboard":        {Canonical: "dashboard"},
	"process":          {Canonical: "process"},
	"default":          {Canonical: "default"},
	"closing":          {Canonical: "closing"},
	"end":              {Canonical: "closing"},
}

func LookupSlideType(name string) (SlideType, bool) { v, ok := slideTypes[name]; return v, ok }
func IsKnownSlideType(name string) bool              { _, ok := LookupSlideType(name); return ok }
func IsTitleSlide(name string) bool                  { v, ok := LookupSlideType(name); return ok && v.Title }
func IsContentSlide(name string) bool                { v, ok := LookupSlideType(name); return name == "" || (ok && v.Content) }
func CanonicalSlideType(name string) string {
	if v, ok := LookupSlideType(name); ok {
		return v.Canonical
	}
	return name
}
func OpensSection(name string) bool { v, ok := LookupSlideType(name); return ok && v.OpensSection }

func KnownSlideTypes() []string {
	names := make([]string, 0, len(slideTypes))
	for name := range slideTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DeckSection is derived data; it is intentionally not serialized in the
// AST. Index and slide offsets are zero-based and FirstSlide/LastSlide are
// inclusive.
type DeckSection struct {
	Index                 int
	Title                 string
	FirstSlide, LastSlide int
}

func DeckSections(doc *ast.AST) []DeckSection {
	if doc == nil {
		return nil
	}
	var sections []DeckSection
	for i := range doc.ContentBlocks {
		block := &doc.ContentBlocks[i]
		if !OpensSection(block.BlockType) {
			continue
		}
		if len(sections) > 0 {
			sections[len(sections)-1].LastSlide = i - 1
		}
		title, _ := block.SectionTitle()
		sections = append(sections, DeckSection{Index: len(sections), Title: title, FirstSlide: i, LastSlide: len(doc.ContentBlocks) - 1})
	}
	return sections
}

// SectionIndexBySlide returns -1 for slides outside an explicit section.
func SectionIndexBySlide(doc *ast.AST) []int {
	if doc == nil {
		return nil
	}
	result := make([]int, len(doc.ContentBlocks))
	for i := range result {
		result[i] = -1
	}
	for _, section := range DeckSections(doc) {
		for i := section.FirstSlide; i <= section.LastSlide; i++ {
			result[i] = section.Index
		}
	}
	return result
}
