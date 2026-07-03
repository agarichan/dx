// Package selfupdate replaces the running dx binary with the latest GitHub
// release asset (dx-<os>-<arch>), verified against the release's SHA256SUMS.
package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Options struct {
	Repo    string // e.g. "agarichan/dx"
	Current string // running version ("dev" for non-release builds)
	Force   bool   // allow overwriting a non-release ("dev") build

	// Test seams; zero values mean production defaults.
	APIBase string // default https://api.github.com
	DLBase  string // default https://github.com
	OS      string // default runtime.GOOS
	Arch    string // default runtime.GOARCH
	ExePath string // default os.Executable()
	Client  *http.Client
}

func (o *Options) defaults() error {
	if o.APIBase == "" {
		o.APIBase = "https://api.github.com"
	}
	if o.DLBase == "" {
		o.DLBase = "https://github.com"
	}
	if o.OS == "" {
		o.OS = runtime.GOOS
	}
	if o.Arch == "" {
		o.Arch = runtime.GOARCH
	}
	if o.Client == nil {
		o.Client = http.DefaultClient
	}
	if o.ExePath == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate executable: %w", err)
		}
		o.ExePath = exe
	}
	return nil
}

func (o Options) get(url string) ([]byte, error) {
	resp, err := o.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// latestTag asks the GitHub API for the latest release tag (e.g. "v0.2.0").
func (o Options) latestTag() (string, error) {
	b, err := o.get(o.APIBase + "/repos/" + o.Repo + "/releases/latest")
	if err != nil {
		return "", fmt.Errorf("query latest release: %w", err)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(b, &rel); err != nil {
		return "", fmt.Errorf("parse release JSON: %w", err)
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("no tag_name in latest release")
	}
	return rel.TagName, nil
}

// wantSum extracts the asset's checksum from a SHA256SUMS body.
func wantSum(sums, asset string) (string, error) {
	for _, line := range strings.Split(sums, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[1] == asset {
			return f[0], nil
		}
	}
	return "", fmt.Errorf("no SHA256SUMS entry for %s", asset)
}

// Run updates the binary at ExePath to the latest release. The original file
// is untouched unless the downloaded asset passes checksum verification; the
// final replace is an atomic rename in the same directory.
func Run(o Options, stdout io.Writer) error {
	if err := o.defaults(); err != nil {
		return err
	}
	tag, err := o.latestTag()
	if err != nil {
		return err
	}
	latest := strings.TrimPrefix(tag, "v")
	if latest == o.Current {
		fmt.Fprintf(stdout, "dx %s is up to date\n", o.Current)
		return nil
	}
	if o.Current == "dev" && !o.Force {
		return fmt.Errorf("this is not a release build (version dev); use `dx update --force` to overwrite it with %s", tag)
	}

	asset := "dx-" + o.OS + "-" + o.Arch
	dl := o.DLBase + "/" + o.Repo + "/releases/download/" + tag + "/"
	sums, err := o.get(dl + "SHA256SUMS")
	if err != nil {
		return fmt.Errorf("download SHA256SUMS: %w", err)
	}
	want, err := wantSum(string(sums), asset)
	if err != nil {
		return err
	}
	bin, err := o.get(dl + asset)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset, err)
	}
	sum := sha256.Sum256(bin)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("checksum mismatch for %s: want %s got %s", asset, want, got)
	}

	// Write next to the target so the rename stays on one filesystem (atomic).
	tmp, err := os.CreateTemp(filepath.Dir(o.ExePath), ".dx-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), o.ExePath); err != nil {
		return fmt.Errorf("replace %s: %w", o.ExePath, err)
	}
	fmt.Fprintf(stdout, "updated: %s -> %s (%s)\n", o.Current, latest, o.ExePath)
	return nil
}
