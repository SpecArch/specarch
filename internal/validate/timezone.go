package validate

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // the database itself, for a host that keeps none

	"github.com/SpecArch/specarch/internal/source"
)

// checkTimeZone reads the specification's timeZone, the name of the IANA
// Time Zone Database a date or a wall-clock reading meets the clock in:
// date() of an instant gives the instant's calendar date in it, and in UTC
// when none is declared, which zoneWarnings reports at the call (ADR-193).
// A name the database does not hold is time_zone at the key, and the date
// is UTC's until the key is corrected.
func (c *checker) checkTimeZone(d *design) {
	c.zone = time.UTC
	n := source.Child(d.root, "timeZone")
	if n == nil || !source.IsScalar(n) {
		return
	}
	c.zoneDeclared = true
	loc := zoneLocation(source.Str(n))
	if loc == nil {
		c.add(n, "/timeZone", RuleTimeZone, "timeZone %s is not a name of the IANA Time Zone Database; write one as the database spells it, such as Asia/Jakarta or UTC", source.Str(n))
		return
	}
	c.zone = loc
}

// zoneName is the form of a name of the database: parts of letters, digits,
// underscores, plus and minus, each begun with a capital, joined by slashes
// (Asia/Jakarta, Etc/GMT+7, America/Port_of_Spain, UTC). The capital keeps
// out asia/jakarta, which a case-insensitive file system would otherwise
// read on one host and not on another.
var zoneName = regexp.MustCompile(`^[A-Z][A-Za-z0-9_+-]*(/[A-Z][A-Za-z0-9_+-]*)*$`)

// zoneSources are the folders the time package reads the database from
// before its embedded copy, in its order.
var zoneSources = []string{"/usr/share/zoneinfo/", "/usr/share/lib/zoneinfo/", "/usr/lib/locale/TZ/", "/etc/zoneinfo"}

// zoneLocation reads a timeZone: a name of the database, spelled as the
// database spells it. It is nil when the name is none. Local, the host's
// own zone, is no name of the database, and a name a folder of the host's
// database holds in another spelling is refused, since the time package
// would read it on a case-insensitive file system alone; the Swift build
// reads the same folders by the same rule.
func zoneLocation(name string) *time.Location {
	if !zoneName.MatchString(name) || name == "Local" {
		return nil
	}
	if held, exact := zoneSpelled(name); held && !exact {
		return nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil
	}
	return loc
}

// zoneSpelled reports whether a folder of the host's database holds the
// name, read case-insensitively, and whether it holds it spelled as given.
func zoneSpelled(name string) (held, exact bool) {
	for _, dir := range zoneSources {
		path, found, spelled := dir, true, true
		for _, part := range strings.Split(name, "/") {
			entries, err := os.ReadDir(path)
			if err != nil {
				found = false
				break
			}
			match := ""
			for _, e := range entries {
				if e.Name() == part {
					match = part
					break
				}
				if strings.EqualFold(e.Name(), part) {
					match = e.Name()
				}
			}
			if match == "" {
				found = false
				break
			}
			if match != part {
				spelled = false
			}
			path = filepath.Join(path, match)
		}
		if found {
			return true, spelled
		}
	}
	return false, false
}
