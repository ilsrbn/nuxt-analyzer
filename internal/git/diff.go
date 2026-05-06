package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type execFunc func(dir, name string, args ...string) ([]byte, error)

type Differ struct {
	ProjectRoot string
	exec        execFunc
}

func New(projectRoot string) *Differ {
	return &Differ{
		ProjectRoot: projectRoot,
		exec:        defaultExec,
	}
}

func defaultExec(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf("run %s %s in %s: %w: %s", name, strings.Join(args, " "), dir, err, msg)
	}

	return []byte(stdout.String()), nil
}

func (d *Differ) ChangedFiles(base, head string) ([]string, error) {
	if d == nil {
		return nil, fmt.Errorf("nil Differ")
	}

	if strings.HasPrefix(base, "-") || strings.HasPrefix(head, "-") {
		return nil, fmt.Errorf("invalid ref: refs must not start with '-'")
	}

	execFn := d.exec
	if execFn == nil {
		execFn = defaultExec
	}

	out, err := execFn(d.ProjectRoot, "git", "diff", "--name-only", base+".."+head)
	if err != nil {
		return nil, err
	}

	raw := strings.ReplaceAll(string(out), "\r\n", "\n")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}, nil
	}

	lines := strings.Split(raw, "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isGeneratedPath(line) {
			continue
		}
		files = append(files, filepath.ToSlash(line))
	}

	return files, nil
}

func isGeneratedPath(p string) bool {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if p == "" {
		return true
	}

	if strings.HasSuffix(p, ".d.ts") {
		return true
	}

	switch {
	case strings.HasPrefix(p, "node_modules/"):
		return true
	case strings.HasPrefix(p, ".nuxt/"):
		return true
	case strings.HasPrefix(p, ".output/"):
		return true
	case strings.HasPrefix(p, "dist/"):
		return true
	default:
		return false
	}
}
