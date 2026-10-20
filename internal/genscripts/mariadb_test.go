package genscripts

import (
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/idioms"
)

// TestMariaNowIsTheIdiomsCurrentTime keeps the instant a MariaDB script
// writes the same as the default the type-rendering idiom gives a
// date-time column on MariaDB, so the two never sort apart.
func TestMariaNowIsTheIdiomsCurrentTime(t *testing.T) {
	data, err := idioms.Shipped.ReadFile("type-rendering/type-rendering.specarch-idiom.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Parts map[string]struct {
			Stack map[string]struct {
				Names map[string]string `yaml:"names"`
			} `yaml:"stack"`
		} `yaml:"parts"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	got := doc.Parts["defaults"].Stack["mariadb"].Names["currentTime"]
	if want := "(" + mariaNow + ")"; got != want {
		t.Errorf("type-rendering names %q for currentTime on MariaDB, and the scripts write %q", got, want)
	}
}
