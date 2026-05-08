package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const releaseBaseURL = "https://github.com/ilsrbn/nuxt-analyzer/releases/download"

func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client := http.DefaultClient

	latest, err := fetchLatestVersion(ctx, client)
	if err != nil {
		return fmt.Errorf("check latest release: %w", err)
	}

	if normalizeVersion(version) == normalizeVersion(latest) {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "already at latest version")
		return err
	}

	tmpDir, err := os.MkdirTemp("", "impact-map-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archiveName := fmt.Sprintf("impact-map_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	archiveURL := fmt.Sprintf("%s/%s/%s", releaseBaseURL, latest, archiveName)
	checksumsURL := fmt.Sprintf("%s/%s/checksums.txt", releaseBaseURL, latest)

	archivePath := filepath.Join(tmpDir, archiveName)
	if err := downloadFile(ctx, client, archiveURL, archivePath); err != nil {
		return fmt.Errorf("download archive: %w", err)
	}

	checksums, err := downloadBytes(ctx, client, checksumsURL)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	if err := verifyArchiveChecksum(archivePath, checksums); err != nil {
		return fmt.Errorf("verify checksum: %w", err)
	}

	extractedBinary, err := extractBinaryFromTarGz(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	target, err := os.Executable()
	if err != nil {
		return fmt.Errorf("current executable: %w", err)
	}
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	if err := replaceExecutable(target, extractedBinary); err != nil {
		return fmt.Errorf("replace executable: %w", err)
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Upgraded %s → %s\n", version, latest)
	return err
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string) error {
	data, err := downloadBytes(ctx, client, url)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

func downloadBytes(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "impact-map/"+version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func verifyArchiveChecksum(archivePath string, checksums []byte) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, archive); err != nil {
		return err
	}
	got := hex.EncodeToString(hash.Sum(nil))
	name := filepath.Base(archivePath)

	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := fields[0]
		file := strings.TrimPrefix(fields[len(fields)-1], "*")
		if file != name {
			continue
		}
		if !strings.EqualFold(sum, got) {
			return fmt.Errorf("%s checksum mismatch", name)
		}
		return nil
	}

	return fmt.Errorf("%s not found in checksums.txt", name)
}

func extractBinaryFromTarGz(archivePath, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		base := filepath.Base(header.Name)
		if base != "impact-map" && base != "impact-map.exe" {
			continue
		}

		target := filepath.Join(destDir, base)
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		cleanTarget := filepath.Clean(target)
		if !strings.HasPrefix(cleanTarget, cleanDest) {
			return "", fmt.Errorf("archive contains unsafe path %q", header.Name)
		}

		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return "", err
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		return target, nil
	}

	return "", fmt.Errorf("archive did not contain impact-map binary")
}

func replaceExecutable(target, source string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	next := target + ".new"
	if err := os.WriteFile(next, data, 0o755); err != nil {
		return err
	}
	if err := os.Rename(next, target); err != nil {
		_ = os.Remove(next)
		return err
	}
	return os.Chmod(target, 0o755)
}
