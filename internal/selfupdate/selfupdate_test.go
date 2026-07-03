package selfupdate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRelease serves the GitHub API + download endpoints for one release.
func fakeRelease(t *testing.T, tag string, binary []byte) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(binary)
	sums := hex.EncodeToString(sum[:]) + "  dx-testos-testarch\n"
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/agarichan/dx/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name": %q}`, tag)
	})
	mux.HandleFunc("/agarichan/dx/releases/download/"+tag+"/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, sums)
	})
	mux.HandleFunc("/agarichan/dx/releases/download/"+tag+"/dx-testos-testarch", func(w http.ResponseWriter, r *http.Request) {
		w.Write(binary)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func opts(srv *httptest.Server, exe, current string, force bool) Options {
	return Options{
		Repo: "agarichan/dx", Current: current, Force: force,
		APIBase: srv.URL, DLBase: srv.URL,
		OS: "testos", Arch: "testarch",
		ExePath: exe, Client: srv.Client(),
	}
}

func writeExe(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "dx")
	if err := os.WriteFile(exe, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestRun_UpdatesBinary(t *testing.T) {
	srv := fakeRelease(t, "v0.2.0", []byte("new-binary"))
	exe := writeExe(t)
	var out bytes.Buffer
	if err := Run(opts(srv, exe, "0.1.0", false), &out); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(exe)
	if string(b) != "new-binary" {
		t.Fatalf("exe content = %q", b)
	}
	fi, _ := os.Stat(exe)
	if fi.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v", fi.Mode())
	}
	if !strings.Contains(out.String(), "0.1.0 -> 0.2.0") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestRun_AlreadyUpToDate(t *testing.T) {
	srv := fakeRelease(t, "v0.2.0", []byte("new-binary"))
	exe := writeExe(t)
	var out bytes.Buffer
	if err := Run(opts(srv, exe, "0.2.0", false), &out); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(exe)
	if string(b) != "old-binary" {
		t.Fatal("exe must be untouched")
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestRun_DevRequiresForce(t *testing.T) {
	srv := fakeRelease(t, "v0.2.0", []byte("new-binary"))
	exe := writeExe(t)
	err := Run(opts(srv, exe, "dev", false), new(bytes.Buffer))
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("err = %v", err)
	}
	// with force it proceeds
	if err := Run(opts(srv, exe, "dev", true), new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(exe)
	if string(b) != "new-binary" {
		t.Fatal("forced update did not replace exe")
	}
}

func TestRun_ChecksumMismatchLeavesExe(t *testing.T) {
	srv := fakeRelease(t, "v0.2.0", []byte("new-binary"))
	exe := writeExe(t)
	o := opts(srv, exe, "0.1.0", false)
	// tamper: point the download at a different body via a wrapping server
	tampered := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "dx-testos-testarch") {
			w.Write([]byte("evil"))
			return
		}
		resp, err := srv.Client().Get(srv.URL + r.URL.Path)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		w.Write(buf.Bytes())
	}))
	t.Cleanup(tampered.Close)
	o.APIBase, o.DLBase, o.Client = tampered.URL, tampered.URL, tampered.Client()

	err := Run(o, new(bytes.Buffer))
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v", err)
	}
	b, _ := os.ReadFile(exe)
	if string(b) != "old-binary" {
		t.Fatal("exe must be untouched on checksum failure")
	}
}

func TestRun_RefusesMiseManagedInstall(t *testing.T) {
	srv := fakeRelease(t, "v9.9.9", []byte("new-binary"))
	dir := filepath.Join(t.TempDir(), "mise", "installs", "github-agarichan-dx", "0.4.0")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "dx")
	os.WriteFile(exe, []byte("old-binary"), 0o755)
	o := opts(srv, exe, "0.4.0", false)
	err := Run(o, new(bytes.Buffer))
	if err == nil || !strings.Contains(err.Error(), "mise") {
		t.Fatalf("err = %v", err)
	}
	// --force does not bypass: mise would fight the replaced binary
	o.Force = true
	if err := Run(o, new(bytes.Buffer)); err == nil {
		t.Fatal("force must not bypass the mise guard")
	}
	b, _ := os.ReadFile(exe)
	if string(b) != "old-binary" {
		t.Fatal("exe must be untouched")
	}
}

func TestRun_MiseDataDirEnvDetected(t *testing.T) {
	srv := fakeRelease(t, "v9.9.9", []byte("new-binary"))
	data := t.TempDir() // custom MISE_DATA_DIR without "mise" in the path
	dir := filepath.Join(data, "installs", "github-agarichan-dx", "0.4.0")
	os.MkdirAll(dir, 0o755)
	exe := filepath.Join(dir, "dx")
	os.WriteFile(exe, []byte("old-binary"), 0o755)
	o := opts(srv, exe, "0.4.0", false)
	o.Getenv = func(k string) string {
		if k == "MISE_DATA_DIR" {
			return data
		}
		return ""
	}
	if err := Run(o, new(bytes.Buffer)); err == nil || !strings.Contains(err.Error(), "mise") {
		t.Fatalf("err = %v", err)
	}
}
