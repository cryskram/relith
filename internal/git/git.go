package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func itoa(n int) string {
	return strconv.Itoa(n)
}

func IsRepo(path string) bool {
	info, err := os.Stat(path + "/.git")
	return err == nil && info.IsDir()
}

func run(ctx context.Context, workdir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workdir
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", ctx.Err()
		}
		return strings.TrimSpace(out.String()), err
	}
	return out.String(), nil
}

func RecentCommits(ctx context.Context, workdir string, max int) (string, error) {
	format := "%H%x1f%ai%x1f%an%x1f%ae%x1f%s%x1e"
	args := []string{"log", "-n", itoa(max), "--pretty=format:" + format}
	return run(ctx, workdir, args...)
}

func FileHistory(ctx context.Context, workdir, file string, max int) (string, error) {
	format := "%H%x1f%ai%x1f%an%x1f%ae%x1f%s%x1e"
	args := []string{"log", "-n", itoa(max), "--pretty=format:" + format, "--follow", "--", file}
	return run(ctx, workdir, args...)
}

func Blame(ctx context.Context, workdir, file string, start, end int) (string, error) {
	args := []string{"blame", "--line-porcelain"}
	if start > 0 && end > 0 {
		args = append(args, "-L", itoa(start)+","+itoa(end))
	}
	args = append(args, "--", file)
	return run(ctx, workdir, args...)
}

func Diff(ctx context.Context, workdir, base, head string, maxStat int) (string, error) {
	statArgs := []string{"diff", "--stat=" + itoa(maxStat), base, head}
	stat, err := run(ctx, workdir, statArgs...)
	if err != nil {
		return "", err
	}
	patchArgs := []string{"diff", "--no-color", base, head}
	patch, err := run(ctx, workdir, patchArgs...)
	if err != nil {
		return "", err
	}
	return stat + "\n" + patch, nil
}