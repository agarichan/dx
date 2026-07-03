package cli

import (
	"bytes"
	"testing"
)

// dx wt は dx worktree の別名 — 引数なし・-h の双方で同一挙動。
func TestWtAlias_MatchesWorktree(t *testing.T) {
	var wtOut, wtErr, fullOut, fullErr bytes.Buffer
	rcWt := Run([]string{"wt"}, &wtOut, &wtErr)
	rcFull := Run([]string{"worktree"}, &fullOut, &fullErr)
	if rcWt != rcFull || wtErr.String() != fullErr.String() || wtOut.String() != fullOut.String() {
		t.Fatalf("wt(%d,%q,%q) != worktree(%d,%q,%q)", rcWt, wtOut.String(), wtErr.String(), rcFull, fullOut.String(), fullErr.String())
	}

	var h1, h2, e1, e2 bytes.Buffer
	if rc := Run([]string{"wt", "-h"}, &h1, &e1); rc != 0 {
		t.Fatalf("wt -h rc=%d", rc)
	}
	Run([]string{"worktree", "-h"}, &h2, &e2)
	if h1.String() != h2.String() {
		t.Fatalf("wt help differs from worktree help")
	}
}
