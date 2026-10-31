// Package ooxml reads the parts of an Office Open XML package (ISO/IEC
// 29500): a zip of XML parts tied together by relationships. The workbook,
// document and presentation readers share it.
package ooxml

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"path"
	"strings"
)

// Files is the package's parts by name.
func Files(z *zip.Reader) map[string]*zip.File {
	files := map[string]*zip.File{}
	for _, f := range z.File {
		files[f.Name] = f
	}
	return files
}

// Relationship is one entry of a part's relationships.
type Relationship struct{ Type, Target string }

// Relationships reads the relationships of a package part, by id, with
// each target resolved against the part's folder.
func Relationships(files map[string]*zip.File, part string) (map[string]Relationship, error) {
	name := path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
	out := map[string]Relationship{}
	if _, ok := files[name]; !ok {
		return out, nil
	}
	var rels struct {
		Items []struct {
			ID     string `xml:"Id,attr"`
			Type   string `xml:"Type,attr"`
			Target string `xml:"Target,attr"`
			Mode   string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if err := Decode(files, name, &rels); err != nil {
		return nil, err
	}
	for _, r := range rels.Items {
		target := r.Target
		if r.Mode != "External" {
			if strings.HasPrefix(target, "/") {
				target = strings.TrimPrefix(target, "/")
			} else {
				target = path.Join(path.Dir(part), target)
			}
		}
		out[r.ID] = Relationship{Type: r.Type, Target: target}
	}
	return out, nil
}

// CoreTitle is the package's dc:title from docProps/core.xml, or "".
func CoreTitle(files map[string]*zip.File) string {
	var core struct {
		Title string `xml:"http://purl.org/dc/elements/1.1/ title"`
	}
	if Decode(files, "docProps/core.xml", &core) != nil {
		return ""
	}
	return strings.TrimSpace(core.Title)
}

// Decode reads one XML part of the package into v.
func Decode(files map[string]*zip.File, name string, v any) error {
	f, ok := files[name]
	if !ok {
		return fmt.Errorf("the package holds no %s", name)
	}
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("%s: %v", name, err)
	}
	defer rc.Close()
	if err := xml.NewDecoder(rc).Decode(v); err != nil {
		return fmt.Errorf("%s does not parse as XML: %v", name, err)
	}
	return nil
}
