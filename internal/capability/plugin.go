package capability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PluginPrefix starts the name of every generator plug-in on PATH.
const PluginPrefix = "specarch-gen-"

// PluginFormat is the version of the answer a plug-in gives to
// --capabilities.
const PluginFormat = "1.0.0"

// Plugin is what a generator plug-in says about itself when run with
// --capabilities: the target and stack it writes, what it writes and what
// it leaves to people.
type Plugin struct {
	Format string `json:"specarchPluginCapabilities"`
	// Plugin is the executable's name, specarch-gen-<target> or
	// specarch-gen-<target>-<stack>.
	Plugin string `json:"plugin"`
	Target string `json:"target"`
	// Stack is the framework or language the plug-in is chosen by, or ""
	// for the plug-in of every stack with none of its own.
	Stack   string `json:"stack,omitempty"`
	Version string `json:"version"`
	Summary string `json:"summary"`
	// Generates are the files and kinds of code it writes.
	Generates []string `json:"generates"`
	// DoesNotGenerate is what it leaves to people or to another target,
	// so nobody waits for output it never writes.
	DoesNotGenerate []string `json:"doesNotGenerate"`
}

// Name is the executable name generate looks the plug-in up by.
func (p Plugin) Name() string {
	if p.Stack == "" {
		return PluginPrefix + p.Target
	}
	return PluginPrefix + p.Target + "-" + p.Stack
}

// Serve answers a plug-in's command-line arguments: --capabilities prints
// what the plug-in says about itself, as JSON; anything else is a usage
// error. A plug-in calls it when it is given any argument, since specarch
// runs it with none.
func Serve(args []string, stdout, stderr io.Writer, p Plugin) int {
	p.Format = PluginFormat
	p.Plugin = p.Name()
	if len(args) == 1 && args[0] == "--capabilities" {
		out, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "%s: cannot encode the capabilities: %v\n", p.Plugin, err)
			return 2
		}
		if _, err := stdout.Write(append(out, '\n')); err != nil {
			fmt.Fprintf(stderr, "%s: cannot write the capabilities: %v\n", p.Plugin, err)
			return 2
		}
		return 0
	}
	fmt.Fprintf(stderr, "%s takes no argument but --capabilities, given %q; specarch generate %s runs it with none and the request on standard input\n", p.Plugin, strings.Join(args, " "), p.Target)
	return 2
}

// Found is one plug-in found on PATH: its path, and what it answered to
// --capabilities, or why it answered nothing usable.
type Found struct {
	Path         string  `json:"path"`
	Capabilities *Plugin `json:"capabilities,omitempty"`
	// Error is why the plug-in gave no capabilities: it failed, timed
	// out, answered something that is not the shape, or named another
	// executable than its own. A plug-in built before --capabilities
	// existed lands here; it still generates.
	Error string `json:"error,omitempty"`
}

// Name is the executable's name.
func (f Found) Name() string { return filepath.Base(f.Path) }

// Gather finds every plug-in on the search path, the first of each name in
// path order as generate runs it, and asks each for its capabilities,
// with its standard input empty and a time limit, so an older plug-in
// that waits for a request answers at once with an error.
func Gather(searchPath string, limit time.Duration) []Found {
	seen := map[string]bool{}
	var out []Found
	for _, dir := range filepath.SplitList(searchPath) {
		if dir == "" {
			dir = "."
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, PluginPrefix) || seen[name] || e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil || info.Mode()&0o111 == 0 {
				continue
			}
			seen[name] = true
			out = append(out, ask(filepath.Join(dir, name), limit))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

func ask(exe string, limit time.Duration) Found {
	f := Found{Path: exe}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "--capabilities")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		f.Error = fmt.Sprintf("gave no answer to --capabilities within %s", limit)
		return f
	}
	if err != nil {
		f.Error = fmt.Sprintf("--capabilities failed (%v): %s", err, firstLine(stderr.String()))
		return f
	}
	var p Plugin
	if err := json.Unmarshal(stdout.Bytes(), &p); err != nil || p.Format == "" {
		f.Error = fmt.Sprintf("--capabilities answered something that is not a plug-in's capabilities: %s", firstLine(stdout.String()))
		return f
	}
	if p.Name() != f.Name() {
		f.Error = fmt.Sprintf("says it is %s (target %s, stack %q), but generate finds it as %s", p.Name(), p.Target, p.Stack, f.Name())
		return f
	}
	f.Capabilities = &p
	return f
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	if s == "" {
		return "(nothing)"
	}
	return s
}
