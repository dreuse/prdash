package update

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewerIgnoresAnUnknownLocalBuild(t *testing.T) {
	if Newer("v0.2.0", "dev") {
		t.Error("a local build has no version to compare, so it must not nag")
	}
	if Newer("", "v0.1.0") {
		t.Error("an empty tag means the check failed, not that an update exists")
	}
}

func TestNewerComparesNumbersNotText(t *testing.T) {
	if !Newer("v0.10.0", "v0.9.0") {
		t.Error("0.10.0 comes after 0.9.0; a string compare gets this backwards")
	}
	if Newer("v0.9.0", "v0.10.0") {
		t.Error("0.9.0 is older than 0.10.0")
	}
}

func TestNewerRejectsTheSameOrOlderRelease(t *testing.T) {
	if Newer("v0.1.0", "v0.1.0") {
		t.Error("the current release is not an update")
	}
	if Newer("v0.1.0", "v0.2.0") {
		t.Error("running ahead of the latest release is not an update")
	}
}

func TestNewerTreatsAPrereleaseAsOlderThanItsRelease(t *testing.T) {
	if !Newer("v1.0.0", "v1.0.0-rc1") {
		t.Error("the final release supersedes its own release candidate")
	}
	if Newer("v1.0.0-rc1", "v1.0.0") {
		t.Error("a release candidate must not pull a finished release backwards")
	}
}

func TestAssetNameMatchesTheReleaseWorkflow(t *testing.T) {
	if got := assetName("v0.1.0", "darwin", "arm64"); got != "prdash_v0.1.0_darwin_arm64.tar.gz" {
		t.Errorf("asset name must match what release.yml uploads, got %q", got)
	}
	if got := assetName("v0.1.0", "windows", "amd64"); got != "prdash_v0.1.0_windows_amd64.zip" {
		t.Errorf("windows ships a zip, got %q", got)
	}
}

func TestVerifyAcceptsThePublishedChecksum(t *testing.T) {
	dir := t.TempDir()
	archive := writeArchive(t, dir, "prdash_v0.2.0_test.tar.gz", "new binary")
	sums := writeChecksums(t, dir, archive)

	if err := verify(archive, sums); err != nil {
		t.Fatalf("an untouched download must verify: %v", err)
	}
}

func TestVerifyRejectsATamperedArchive(t *testing.T) {
	dir := t.TempDir()
	archive := writeArchive(t, dir, "prdash_v0.2.0_test.tar.gz", "new binary")
	sums := writeChecksums(t, dir, archive)
	if err := os.WriteFile(archive, []byte("swapped payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := verify(archive, sums); err == nil {
		t.Fatal("a download that does not match checksums.txt must never be installed")
	}
}

func TestVerifyRejectsAnArchiveWithNoPublishedChecksum(t *testing.T) {
	dir := t.TempDir()
	archive := writeArchive(t, dir, "prdash_v0.2.0_test.tar.gz", "new binary")
	sums := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(sums, []byte("deadbeef  something_else.tar.gz\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := verify(archive, sums); err == nil {
		t.Fatal("an unlisted asset is unverified, so it must not install")
	}
}

func TestInstallReplacesTheRunningBinaryInPlace(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, binaryName())
	if err := os.WriteFile(dest, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	archive := writeArchive(t, t.TempDir(), "prdash_v0.2.0_test.tar.gz", "new binary")

	if err := install(dest, archive); err != nil {
		t.Fatalf("install: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new binary" {
		t.Errorf("the new build should sit at the old path, got %q", got)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("the installed binary must stay executable, got %v", info.Mode().Perm())
	}
	left, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 {
		t.Errorf("install should leave no scratch files behind, got %v", left)
	}
}

func TestInstallRefusesAnArchiveWithoutTheBinary(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, binaryName())
	if err := os.WriteFile(dest, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "prdash_v0.2.0_test.tar.gz")
	writeTarGz(t, archive, "prdash_v0.2.0_test/README.md", "docs only")

	if err := install(dest, archive); err == nil {
		t.Fatal("an archive with no binary must not touch the installed one")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old binary" {
		t.Errorf("a failed update must leave the working binary alone, got %q", got)
	}
}

func writeArchive(t *testing.T, dir, name, payload string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	writeTarGz(t, path, strings.TrimSuffix(name, ".tar.gz")+"/"+binaryName(), payload)
	return path
}

func writeTarGz(t *testing.T, path, entry, payload string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: entry, Mode: 0o755, Size: int64(len(payload)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeChecksums(t *testing.T, dir, archive string) string {
	t.Helper()
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	path := filepath.Join(dir, "checksums.txt")
	line := hex.EncodeToString(sum[:]) + "  " + filepath.Base(archive) + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
