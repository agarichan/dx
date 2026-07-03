package raycast

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
)

// buildOKRe is the line `ray develop` prints once the extension is built and
// imported into Raycast (same marker as raycast/scripts/import.mjs).
var buildOKRe = regexp.MustCompile(`(?i)built extension successfully`)

// pump copies r to w line-by-line and fires onMarker once when a line matches
// buildOKRe. Line-based scanning so chunk boundaries cannot split the marker.
func pump(r io.Reader, w io.Writer, onMarker func()) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	fired := false
	for sc.Scan() {
		line := sc.Text()
		fmt.Fprintln(w, line)
		if !fired && buildOKRe.MatchString(line) {
			fired = true
			onMarker()
		}
	}
}

// Tools are the external commands Install depends on, injected for tests.
type Tools struct {
	LookPath  func(file string) (string, error)
	NpmCI     func(dir string, stdout, stderr io.Writer) error
	RayImport func(dir string, stdout, stderr io.Writer) error
}

// Install extracts the embedded extension into dir, installs runtime deps and
// imports the extension into Raycast via a one-shot `ray develop`.
func Install(src fs.FS, dir string, t Tools, stdout, stderr io.Writer) error {
	if _, err := t.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found in PATH — Node.js is required (same prerequisite as portless)")
	}
	if err := Extract(src, dir); err != nil {
		return fmt.Errorf("extract extension: %w", err)
	}
	fmt.Fprintf(stdout, "extension source -> %s\n", dir)
	if err := t.NpmCI(dir, stdout, stderr); err != nil {
		return fmt.Errorf("npm ci: %w", err)
	}
	if err := t.RayImport(dir, stdout, stderr); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "✓ imported into Raycast — the \"List dx Services\" command is available.")
	fmt.Fprintln(stdout, "  to update after a dx upgrade, re-run: dx raycast install")
	return nil
}

// Uninstall removes the extracted extension directory. Raycast offers no CLI
// to drop the extension entry itself, so print how to do it in the UI.
func Uninstall(dir string, stdout io.Writer) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Fprintf(stdout, "nothing to remove (%s does not exist)\n", dir)
	} else {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "removed %s\n", dir)
	}
	fmt.Fprintln(stdout, "To remove the entry from Raycast: select the extension → ⌘K → Remove Extension.")
	return nil
}
