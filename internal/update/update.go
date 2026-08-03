package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	Repo         = "dreuse/prdash"
	checksumFile = "checksums.txt"
	ApplyTimeout = 3 * time.Minute
	devVersion   = "dev"
)

var Version string

var ghBin = "gh"

func Current() string {
	if Version != "" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return devVersion
}

func Latest(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, ghBin,
		"release", "view", "--repo", Repo, "--json", "tagName", "--jq", ".tagName").Output()
	if err != nil {
		return "", fmt.Errorf("check for updates: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func Apply(ctx context.Context, tag string) error {
	dest, err := runningBinary()
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "prdash-update")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	asset := assetName(tag, runtime.GOOS, runtime.GOARCH)
	out, err := exec.CommandContext(ctx, ghBin, "release", "download", tag,
		"--repo", Repo, "--pattern", asset, "--pattern", checksumFile, "--dir", dir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("download %s: %w: %s", asset, err, strings.TrimSpace(string(out)))
	}

	archive := filepath.Join(dir, asset)
	if err := verify(archive, filepath.Join(dir, checksumFile)); err != nil {
		return err
	}
	return install(dest, archive)
}

func runningBinary() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved, nil
	}
	return path, nil
}

func assetName(tag, goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("prdash_%s_%s_%s%s", tag, goos, goarch, ext)
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "prdash.exe"
	}
	return "prdash"
}

func verify(archive, sums string) error {
	sum, err := sha256File(archive)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(sums)
	if err != nil {
		return err
	}

	name := filepath.Base(archive)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		if fields[0] != sum {
			return fmt.Errorf("%s does not match its published checksum, refusing to install it", name)
		}
		return nil
	}
	return fmt.Errorf("%s has no entry in %s, refusing to install it", name, checksumFile)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func install(dest, archive string) error {
	staged, err := extract(archive, filepath.Dir(dest))
	if err != nil {
		return err
	}
	defer os.Remove(staged)

	if runtime.GOOS == "windows" {
		superseded := dest + ".old"
		os.Remove(superseded)
		if err := os.Rename(dest, superseded); err != nil {
			return installErr(dest, err)
		}
	}
	if err := os.Rename(staged, dest); err != nil {
		return installErr(dest, err)
	}
	return nil
}

func installErr(dest string, err error) error {
	if os.IsPermission(err) {
		return fmt.Errorf("cannot write %s: %w\nrerun with write access to %s, or update through whatever installed it",
			dest, err, filepath.Dir(dest))
	}
	return fmt.Errorf("install %s: %w", dest, err)
}

func extract(archive, stageDir string) (string, error) {
	if strings.HasSuffix(archive, ".zip") {
		return extractZip(archive, stageDir)
	}
	return extractTar(archive, stageDir)
}

func extractTar(archive, stageDir string) (string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if h.Typeflag == tar.TypeReg && filepath.Base(h.Name) == binaryName() {
			return stageBesideTarget(stageDir, tr)
		}
	}
	return "", missingBinary(archive)
}

func extractZip(archive, stageDir string) (string, error) {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, entry := range r.File {
		if entry.FileInfo().IsDir() || filepath.Base(entry.Name) != binaryName() {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		return stageBesideTarget(stageDir, rc)
	}
	return "", missingBinary(archive)
}

func missingBinary(archive string) error {
	return fmt.Errorf("%s holds no %s binary", filepath.Base(archive), binaryName())
}

func stageBesideTarget(dir string, src io.Reader) (string, error) {
	f, err := os.CreateTemp(dir, "."+binaryName()+".*")
	if err != nil {
		return "", installErr(filepath.Join(dir, binaryName()), err)
	}
	path := f.Name()
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	if err := os.Chmod(path, 0o755); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

func Newer(latest, current string) bool {
	l, ok := parseVersion(latest)
	if !ok {
		return false
	}
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	return l.after(c)
}

type version struct {
	num [3]int
	pre string
}

func parseVersion(s string) (version, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if build := strings.IndexByte(s, '+'); build >= 0 {
		s = s[:build]
	}
	var v version
	if pre := strings.IndexByte(s, '-'); pre >= 0 {
		v.pre, s = s[pre+1:], s[:pre]
	}
	parts := strings.Split(s, ".")
	if len(parts) > len(v.num) {
		return version{}, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return version{}, false
		}
		v.num[i] = n
	}
	return v, true
}

func (v version) after(o version) bool {
	for i := range v.num {
		if v.num[i] != o.num[i] {
			return v.num[i] > o.num[i]
		}
	}
	switch {
	case v.pre == o.pre:
		return false
	case v.pre == "":
		return true
	case o.pre == "":
		return false
	}
	return v.pre > o.pre
}
