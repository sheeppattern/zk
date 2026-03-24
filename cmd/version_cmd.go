package cmd

import (
	"bufio"
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
	"time"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show zk version information",
	Example: `  zk version
  zk version --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		info := struct {
			Version  string `json:"version" yaml:"version"`
			OS       string `json:"os" yaml:"os"`
			Arch     string `json:"arch" yaml:"arch"`
			GoVer    string `json:"go_version" yaml:"go_version"`
		}{
			Version: Version,
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
			GoVer:   runtime.Version(),
		}

		f := getFormatter()
		switch f.Format {
		case "json":
			return f.PrintJSON(info)
		case "yaml":
			return f.PrintYAML(info)
		default:
			fmt.Fprintf(os.Stdout, "zk %s (%s/%s, %s)\n", info.Version, info.OS, info.Arch, info.GoVer)
			return nil
		}
	},
}

// ghRelease represents a GitHub release (minimal fields).
type ghRelease struct {
	TagName string    `json:"tag_name"`
	HTMLURL string    `json:"html_url"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

const ghReleaseAPI = "https://api.github.com/repos/sheeppattern/zk/releases/latest"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update zk to the latest version",
	Long:  "Downloads the latest release from GitHub and replaces the current binary.",
	Example: `  zk update
  zk update --check`,
	RunE: func(cmd *cobra.Command, args []string) error {
		checkOnly, _ := cmd.Flags().GetBool("check")

		statusf("checking for updates...")

		release, err := fetchLatestRelease()
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}

		latest := strings.TrimPrefix(release.TagName, "v")
		current := Version

		if latest == current {
			statusf("already up to date (v%s)", current)
			return nil
		}

		statusf("new version available: v%s → v%s", current, latest)

		if checkOnly {
			statusf("run 'zk update' to install")
			return nil
		}

		// Find matching asset for current OS/arch.
		assetName := fmt.Sprintf("zk_%s_%s", runtime.GOOS, runtime.GOARCH)
		if runtime.GOOS == "windows" {
			assetName += ".exe"
		}

		var downloadURL string
		for _, a := range release.Assets {
			if strings.Contains(strings.ToLower(a.Name), strings.ToLower(assetName)) {
				downloadURL = a.BrowserDownloadURL
				break
			}
		}

		if downloadURL == "" {
			statusf("no pre-built binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
			statusf("install manually: go install github.com/sheeppattern/zk@v%s", latest)
			return nil
		}

		statusf("downloading %s...", assetName)

		// Download to temp file.
		resp, err := http.Get(downloadURL)
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
		}

		tmpFile, err := os.CreateTemp("", "zk-update-*")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			tmpFile.Close()
			return fmt.Errorf("download write: %w", err)
		}
		tmpFile.Close()

		// Verify integrity if checksums file is available in release.
		if checksumURL := findChecksumAssetURL(release.Assets); checksumURL != "" {
			expectedHash, err := fetchExpectedHash(checksumURL, assetName)
			if err != nil {
				statusf("warning: checksum verification skipped: %v", err)
			} else {
				actualHash, err := sha256File(tmpPath)
				if err != nil {
					return fmt.Errorf("compute file hash: %w", err)
				}
				if actualHash != expectedHash {
					return fmt.Errorf("integrity check failed: expected sha256 %s, got %s", expectedHash, actualHash)
				}
				statusf("integrity verified (sha256: %s...)", actualHash[:16])
			}
		} else {
			statusf("warning: no checksums file in release, skipping integrity verification")
		}

		// Replace current binary.
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate current binary: %w", err)
		}
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			return fmt.Errorf("resolve binary path: %w", err)
		}

		// On Windows, rename current binary before replacing.
		backupPath := execPath + ".old"
		os.Remove(backupPath)
		if err := os.Rename(execPath, backupPath); err != nil {
			return fmt.Errorf("backup current binary: %w", err)
		}

		if err := copyFile(tmpPath, execPath); err != nil {
			// Restore backup on failure.
			os.Rename(backupPath, execPath)
			return fmt.Errorf("replace binary: %w", err)
		}

		if err := os.Chmod(execPath, 0o755); err != nil {
			debugf("chmod failed: %v", err)
		}

		os.Remove(backupPath)

		statusf("updated to v%s", latest)
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove zk binary and optionally clean up data",
	Long:  "Removes the zk binary. Use --purge to also delete the store and agent skill files.",
	Example: `  zk uninstall
  zk uninstall --purge`,
	RunE: func(cmd *cobra.Command, args []string) error {
		purge, _ := cmd.Flags().GetBool("purge")

		if purge {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("cannot determine home directory: %w", err)
			}

			// Remove store.
			storePath := getStorePath(cmd)
			if storePath != "" {
				statusf("removing store at %s", storePath)
				os.RemoveAll(storePath)
			}

			// Remove agent skill files (global).
			agentPaths := []string{
				filepath.Join(home, ".claude", "skills", "zk"),
				filepath.Join(home, ".gemini", "instructions", "zk.md"),
				filepath.Join(home, ".codex", "instructions", "zk.md"),
			}
			for _, p := range agentPaths {
				if _, err := os.Stat(p); err == nil {
					statusf("removing %s", p)
					os.RemoveAll(p)
				}
			}
		}

		// Remove the binary itself.
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate binary: %w", err)
		}
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			return fmt.Errorf("resolve binary path: %w", err)
		}

		statusf("removing binary at %s", execPath)

		if runtime.GOOS == "windows" {
			// On Windows, can't delete running binary. Schedule removal.
			// Rename to .uninstall and print manual cleanup instruction.
			uninstallPath := execPath + ".uninstall"
			if err := os.Rename(execPath, uninstallPath); err != nil {
				return fmt.Errorf("rename binary: %w", err)
			}
			statusf("binary renamed to %s (delete manually after exit)", uninstallPath)
		} else {
			if err := os.Remove(execPath); err != nil {
				return fmt.Errorf("remove binary: %w", err)
			}
		}

		statusf("zk uninstalled successfully")
		return nil
	},
}

func init() {
	updateCmd.Flags().Bool("check", false, "only check for updates, don't install")
	uninstallCmd.Flags().Bool("purge", false, "also remove store data and agent skill files")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(uninstallCmd)
}

// fetchLatestRelease gets the latest release info from GitHub API.
func fetchLatestRelease() (*ghRelease, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ghReleaseAPI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	return &release, nil
}

// findChecksumAssetURL looks for a checksums file in release assets.
func findChecksumAssetURL(assets []ghAsset) string {
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, "checksum") || strings.Contains(name, "sha256") {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

// fetchExpectedHash downloads a checksums file and extracts the hash for the given asset name.
func fetchExpectedHash(checksumURL, assetName string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(checksumURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		// Format: "<hash>  <filename>" or "<hash> <filename>"
		parts := strings.Fields(line)
		if len(parts) >= 2 && strings.Contains(strings.ToLower(parts[1]), strings.ToLower(assetName)) {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("no checksum found for %s", assetName)
}

// sha256File computes the SHA-256 hash of a file.
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

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
