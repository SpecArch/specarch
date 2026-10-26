package validate

import "testing"

// Both builds hold a timeZone to the database's own spelling and refuse
// the names a platform's library knows beyond it.
func TestZoneNamesAreTheDatabases(t *testing.T) {
	for _, name := range []string{"Asia/Jakarta", "UTC", "Etc/UTC", "EST", "Etc/GMT+7", "America/Argentina/Buenos_Aires"} {
		if zoneLocation(name) == nil {
			t.Errorf("%s: refused, and it is a name of the database", name)
		}
	}
	for _, name := range []string{"asia/jakarta", "Asia/JAKARTA", "utc", "Local", "", "PST", "GMT+7", "WIB", "+07:00", "posixrules"} {
		if zoneLocation(name) != nil {
			t.Errorf("%s: accepted, and it is no name of the database", name)
		}
	}
}
