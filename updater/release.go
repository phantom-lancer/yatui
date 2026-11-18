package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const repository = "phantom-lancer/yatui"

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Body    string  `json:"body"`
	Assets  []Asset `json:"assets"`
}

func newHTTPClient(proxy string, timeout time.Duration) *http.Client {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if strings.TrimSpace(proxy) != "" {
		if parsed, err := url.Parse(proxy); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}

// CheckLatest is called only from the explicit "check updates" action.
// GitHub's API is tried first. If api.github.com is blocked by DNS or a
// network filter, the public releases.atom feed on github.com is used instead.
func CheckLatest(ctx context.Context, currentVersion, proxy string) (*Release, bool, error) {
	client := newHTTPClient(proxy, 12*time.Second)
	defer client.CloseIdleConnections()

	release, apiErr := checkLatestAPI(ctx, client)
	if apiErr != nil {
		var feedErr error
		release, feedErr = checkLatestAtom(ctx, client)
		if feedErr != nil {
			return nil, false, fmt.Errorf("GitHub API failed (%v); releases feed failed: %w", apiErr, feedErr)
		}
	}
	if _, err := versionParts(release.TagName); err != nil {
		return nil, false, fmt.Errorf("invalid release version %q: %w", release.TagName, err)
	}
	newer, err := IsNewer(currentVersion, release.TagName)
	if err != nil {
		return nil, false, err
	}
	return release, newer, nil
}

func checkLatestAPI(ctx context.Context, client *http.Client) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/"+repository+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "YATUI-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API: %s", resp.Status)
	}
	var release Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func checkLatestAtom(ctx context.Context, client *http.Client) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://github.com/"+repository+"/releases.atom", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "YATUI-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub releases feed: %s", resp.Status)
	}
	var feed struct {
		Entries []struct {
			Title string `xml:"title"`
			ID    string `xml:"id"`
			Link  struct {
				Href string `xml:"href,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&feed); err != nil {
		return nil, err
	}
	if len(feed.Entries) == 0 {
		return nil, errors.New("GitHub releases feed is empty")
	}
	entry := feed.Entries[0]
	tag := strings.TrimSpace(entry.Title)
	if tag == "" {
		tag = strings.TrimPrefix(entry.ID, "tag:github.com,2008:Repository/")
		if index := strings.LastIndex(tag, "/"); index >= 0 {
			tag = tag[index+1:]
		}
	}
	if tag == "" {
		return nil, errors.New("GitHub releases feed has no version tag")
	}
	base := "https://github.com/" + repository + "/releases/download/" + tag + "/"
	names := []string{
		"SHA256SUMS",
		"yatui-windows-amd64.exe",
		"yatui-windows-amd64-nomedia.exe",
		"yatui-linux-amd64",
		"yatui-linux-amd64-nomedia",
	}
	release := &Release{TagName: tag, HTMLURL: entry.Link.Href}
	for _, name := range names {
		release.Assets = append(release.Assets, Asset{
			Name: name, BrowserDownloadURL: base + name,
		})
	}
	return release, nil
}

func versionParts(v string) ([3]int, error) {
	var parts [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return parts, errors.New("empty version")
	}
	core := strings.SplitN(v, "-", 2)[0]
	core = strings.SplitN(core, "+", 2)[0]
	values := strings.Split(core, ".")
	if len(values) != 3 {
		return parts, fmt.Errorf("expected major.minor.patch")
	}
	for i, value := range values {
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return parts, fmt.Errorf("invalid version component %q", value)
		}
		parts[i] = n
	}
	return parts, nil
}

// IsNewer compares stable numeric major.minor.patch versions.
func IsNewer(current, latest string) (bool, error) {
	a, err := versionParts(current)
	if err != nil {
		return false, err
	}
	b, err := versionParts(latest)
	if err != nil {
		return false, err
	}
	for i := range a {
		if b[i] != a[i] {
			return b[i] > a[i], nil
		}
	}
	return false, nil
}

func platformAsset(goos, goarch string) (string, error) {
	if goarch != "amd64" {
		return "", fmt.Errorf("automatic updates are not available for %s/%s", goos, goarch)
	}
	switch goos {
	case "windows":
		return "yatui-windows-amd64.exe", nil
	case "linux":
		return "yatui-linux-amd64", nil
	default:
		return "", fmt.Errorf("automatic updates are not available for %s/%s", goos, goarch)
	}
}

func findAsset(release *Release, name string) (string, error) {
	for _, asset := range release.Assets {
		if asset.Name == name && asset.BrowserDownloadURL != "" {
			return asset.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("release asset %q is missing", name)
}

func checksumFor(data []byte, filename string) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == filename {
			sum, err := hex.DecodeString(fields[0])
			if err != nil || len(sum) != sha256.Size {
				return "", fmt.Errorf("invalid SHA256 entry for %s", filename)
			}
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("SHA256SUMS has no entry for %s", filename)
}

// DownloadVerified downloads one platform asset into the executable directory
// and verifies it against the release's SHA256SUMS before returning its path.
func DownloadVerified(ctx context.Context, release *Release, directory, proxy string) (string, error) {
	if release == nil {
		return "", errors.New("no release selected")
	}
	name, err := platformAsset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	binaryURL, err := findAsset(release, name)
	if err != nil {
		return "", err
	}
	checksumURL, err := findAsset(release, "SHA256SUMS")
	if err != nil {
		return "", err
	}

	client := newHTTPClient(proxy, 6*time.Minute)
	defer client.CloseIdleConnections()
	checksumReq, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return "", err
	}
	checksumResp, err := client.Do(checksumReq)
	if err != nil {
		return "", err
	}
	checksumData, readErr := io.ReadAll(io.LimitReader(checksumResp.Body, 64<<10))
	checksumResp.Body.Close()
	if readErr != nil {
		return "", readErr
	}
	if checksumResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum download: %s", checksumResp.Status)
	}
	expected, err := checksumFor(checksumData, name)
	if err != nil {
		return "", err
	}

	binaryReq, err := http.NewRequestWithContext(ctx, http.MethodGet, binaryURL, nil)
	if err != nil {
		return "", err
	}
	binaryResp, err := client.Do(binaryReq)
	if err != nil {
		return "", err
	}
	defer binaryResp.Body.Close()
	if binaryResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("binary download: %s", binaryResp.Status)
	}
	if binaryResp.ContentLength > 128<<20 {
		return "", errors.New("update asset is unexpectedly large")
	}

	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", err
	}
	staged, err := os.CreateTemp(directory, ".yatui-update-*")
	if err != nil {
		return "", err
	}
	stagedPath := staged.Name()
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(stagedPath)
		}
	}()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(staged, hasher), io.LimitReader(binaryResp.Body, (128<<20)+1))
	if err != nil {
		staged.Close()
		return "", err
	}
	if written == 0 || written > 128<<20 {
		staged.Close()
		return "", errors.New("downloaded update has an invalid size")
	}
	if err := staged.Sync(); err != nil {
		staged.Close()
		return "", err
	}
	if err := staged.Close(); err != nil {
		return "", err
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return "", fmt.Errorf("SHA256 mismatch: expected %s, got %s", expected, actual)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(stagedPath, 0755); err != nil {
			return "", err
		}
	}
	keep = true
	return filepath.Clean(stagedPath), nil
}
