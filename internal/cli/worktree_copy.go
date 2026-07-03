package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/agarichan/dx/internal/project"
)

// runCopySteps seeds files from the primary worktree into the new worktree at
// the same relative path. A missing source or an already-existing destination
// is skipped (logged, not an error). Directories are copied recursively;
// symlinks are preserved as symlinks.
func runCopySteps(steps []project.CopyStep, primaryRoot, worktreeRoot string, stdout, stderr io.Writer) error {
	for i, step := range steps {
		src := filepath.Join(primaryRoot, step.From)
		dst := filepath.Join(worktreeRoot, step.From)
		prefix := fmt.Sprintf("> [copy %d/%d] %s", i+1, len(steps), step.From)

		srcInfo, err := os.Lstat(src)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				fmt.Fprintf(stdout, "%s (source missing, skipped)\n", prefix)
				continue
			}
			return fmt.Errorf("worktree.copy[%d] %s: stat source: %w", i, step.From, err)
		}
		if _, err := os.Lstat(dst); err == nil {
			fmt.Fprintf(stdout, "%s (destination exists, skipped)\n", prefix)
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("worktree.copy[%d] %s: stat destination: %w", i, step.From, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("worktree.copy[%d] %s: mkdir parent: %w", i, step.From, err)
		}
		fmt.Fprintln(stdout, prefix)
		if err := copyTree(src, dst, srcInfo); err != nil {
			return fmt.Errorf("worktree.copy[%d] %s: %w", i, step.From, err)
		}
	}
	return nil
}

func copyTree(src, dst string, info fs.FileInfo) error {
	switch mode := info.Mode(); {
	case mode&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case mode.IsDir():
		return copyDir(src, dst, info.Mode().Perm())
	case mode.IsRegular():
		return copyFile(src, dst, info.Mode().Perm())
	default:
		return fmt.Errorf("unsupported source mode %v", mode)
	}
}

func copyDir(src, dst string, perm os.FileMode) error {
	if err := os.MkdirAll(dst, perm); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == src {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case d.IsDir():
			return os.MkdirAll(target, mode.Perm())
		case mode.IsRegular():
			return copyFile(path, target, mode.Perm())
		default:
			return nil
		}
	})
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
