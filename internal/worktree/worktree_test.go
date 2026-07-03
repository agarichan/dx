package worktree

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"worktree-ws-category": "worktree_ws_category",
		"feat/Foo Bar":         "feat_foo_bar",
		"123branch":            "_123branch",
		"":                     "wt",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDBName(t *testing.T) {
	if got := DBName("myapp", "main", true); got != "myapp" {
		t.Errorf("primary DBName = %q", got)
	}
	if got := DBName("myapp", "ws-category", false); got != "myapp_ws_category" {
		t.Errorf("worktree DBName = %q", got)
	}
}

func TestDBName_Truncate63(t *testing.T) {
	long := ""
	for i := 0; i < 80; i++ {
		long += "a"
	}
	got := DBName("myapp", long, false)
	if len(got) > 63 {
		t.Fatalf("DBName len = %d (>63): %q", len(got), got)
	}
}

func TestCheckoutHash_StableAndDistinct(t *testing.T) {
	a := CheckoutHash("/home/u/work/myapp")
	b := CheckoutHash("/home/u/work/myapp")
	c := CheckoutHash("/home/u/work/myapp/.claude/worktrees/ws-category")
	if a != b {
		t.Fatal("hash not stable")
	}
	if a == c {
		t.Fatal("hash not distinct across checkouts")
	}
	if len(a) != 12 {
		t.Fatalf("hash len = %d", len(a))
	}
}

func TestDetect_LinkedWorktree(t *testing.T) {
	// git-dir != git-common-dir => linked worktree
	run := func(args ...string) (string, error) {
		switch {
		case eq(args, "rev-parse", "--show-toplevel"):
			return "/home/u/work/myapp/.claude/worktrees/ws-category", nil
		case eq(args, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir"):
			return "/home/u/work/myapp/.git/worktrees/ws-category\n/home/u/work/myapp/.git", nil
		case eq(args, "symbolic-ref", "--quiet", "--short", "HEAD"):
			return "ws-category", nil
		}
		return "", nil
	}
	info, err := Detect(".", run)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsPrimary {
		t.Fatal("expected linked worktree (not primary)")
	}
	if info.Branch != "ws-category" {
		t.Fatalf("branch = %q", info.Branch)
	}
	if info.PrimaryRoot != "/home/u/work/myapp" {
		t.Fatalf("primary root = %q", info.PrimaryRoot)
	}
}

func TestDetect_PrimaryFromSubdir(t *testing.T) {
	// From a subdir of the primary checkout, --path-format=absolute makes both paths
	// equal. Without it, --git-common-dir would be relative ("../.git") and mismatch.
	run := func(args ...string) (string, error) {
		switch {
		case eq(args, "rev-parse", "--show-toplevel"):
			return "/home/u/work/myapp", nil
		case eq(args, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir"):
			return "/home/u/work/myapp/.git\n/home/u/work/myapp/.git", nil
		case eq(args, "symbolic-ref", "--quiet", "--short", "HEAD"):
			return "main", nil
		}
		return "", nil
	}
	info, err := Detect(".", run)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsPrimary {
		t.Fatal("expected primary checkout")
	}
	if info.Branch != "main" {
		t.Fatalf("branch = %q", info.Branch)
	}
	if info.PrimaryRoot != "/home/u/work/myapp" {
		t.Fatalf("primary root = %q", info.PrimaryRoot)
	}
}

func eq(args []string, want ...string) bool {
	if len(args) != len(want) {
		return false
	}
	for i := range args {
		if args[i] != want[i] {
			return false
		}
	}
	return true
}

func TestSlugCollision(t *testing.T) {
	existing := []string{"feat-x", "main"}
	// "feat/x" は Slug→feat_x で "feat-x"(Slug→feat_x) と衝突
	if with, ok := SlugCollision("feat/x", existing); !ok || with != "feat-x" {
		t.Fatalf("expected collision with feat-x, got %q %v", with, ok)
	}
	// 衝突しない
	if _, ok := SlugCollision("totally-new", existing); ok {
		t.Fatal("unexpected collision")
	}
	// 空 slug 同士（両方 "wt"）も衝突扱い
	if _, ok := SlugCollision("///", []string{"@@@"}); !ok {
		t.Fatal("expected wt/wt collision")
	}
}
