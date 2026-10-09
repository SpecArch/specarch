package main

import (
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/SpecArch/specarch/internal/genbpmn"
	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/genui"
	"github.com/SpecArch/specarch/internal/problems"
	"github.com/SpecArch/specarch/internal/source"
)

// siteParts gathers what the html target shows beside the specification:
// the problems as the problems file lists them, each workflow's diagram as
// specarch-gen-bpmn draws it, and each page's screen as specarch-gen-ui
// writes it, all made in memory with the plug-ins' own logic.
func siteParts(l loaded, folder string) *generate.SiteParts {
	parts := &generate.SiteParts{Workflows: map[string]string{}, Previews: map[string]string{}, NoPreview: map[string]string{}}
	pointers := map[string]string{}
	for _, m := range l.marks(folder) {
		pointers[m.ID] = m.Pointer
	}
	for _, p := range problems.Collect(l.spec, l.diags, l.covered, folder) {
		listed := generate.Listed{ID: p.ID, Severity: string(p.Severity), Rule: p.Rule, Message: p.Message, Line: p.String(), Pointer: pointers[p.ID]}
		if p.Severity == problems.Question {
			listed.Pointer = p.Path
		}
		for _, n := range p.Notes {
			listed.Notes = append(listed.Notes, n.String())
		}
		parts.Problems = append(parts.Problems, listed)
	}

	if len(source.Pairs(source.Child(l.spec.Root, "workflows"))) > 0 {
		req := newPluginRequest(l, "bpmn", folder, l.impls)
		req.Draft, _ = gate(l, "bpmn", true, io.Discard)
		if data, err := json.Marshal(req); err == nil {
			if r, err := genbpmn.Decode(data); err == nil {
				for _, f := range genbpmn.Generate(r).Files {
					if name, ok := strings.CutSuffix(f.Path, ".svg"); ok {
						parts.Workflows[name] = f.Content
					}
				}
			}
		}
	}

	if len(source.Pairs(source.Child(l.spec.Root, "pages"))) == 0 {
		return parts
	}
	var screens []generate.Implementation
	for _, i := range l.impls {
		ui := source.Child(source.Child(i.Node, "targets"), "ui")
		if source.Str(source.Child(ui, "platform")) == "web" && source.Str(source.Child(ui, "framework")) == "plain-javascript" {
			screens = append(screens, i)
			break
		}
	}
	if len(screens) == 0 {
		parts.NoPreviews = "no implementation file names a ui target for the web in plain JavaScript, the screens specarch generate ui writes, so there is nothing to show yet."
		return parts
	}
	req := newPluginRequest(l, "ui", folder, screens)
	data, err := json.Marshal(req)
	if err != nil {
		return parts
	}
	r, err := genui.Decode(data)
	if err != nil {
		return parts
	}
	resp := genui.Generate(r)
	theme := ""
	for _, f := range resp.Files {
		if f.Path == "theme.css" {
			theme = f.Content
		}
	}
	for _, d := range resp.Diagnostics {
		if name, ok := strings.CutPrefix(d.Path, "/pages/"); ok && !strings.Contains(name, "/") {
			parts.NoPreview[source.UnescapeToken(name)] = d.Message + "."
		} else if d.Severity == "error" {
			parts.NoPreviews = "specarch generate ui cannot write the screens: " + d.Message + "."
		}
	}
	for _, f := range resp.Files {
		if name, ok := strings.CutSuffix(f.Path, ".html"); ok {
			parts.Previews[name] = screen(f.Content, theme)
		}
	}
	return parts
}

var (
	scriptTag = regexp.MustCompile(`(?m)^<script[^>]*></script>\n`)
	themeLink = regexp.MustCompile(`(?m)^<link rel="stylesheet" href="theme.css">$`)
)

// screen is a page as specarch-gen-ui writes it, made to stand alone in a
// frame: its style sheet inside it and its scripts left out, so it shows
// the screen without data and makes no request.
func screen(page, theme string) string {
	page = scriptTag.ReplaceAllString(page, "")
	return themeLink.ReplaceAllLiteralString(page, "<style>\n"+theme+"</style>")
}
