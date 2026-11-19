// Package release finds and downloads game data releases published on
// GitHub by doduda (github.com/dofusdude/dofus3-main).
package release

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const DefaultRepo = "dofusdude/dofus3-main"

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type Release struct {
	Tag         string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

func (r *Release) Asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

type Client struct {
	Repo  string
	Token string
	HTTP  *http.Client
}

func NewClient(repo, token string) *Client {
	if repo == "" {
		repo = DefaultRepo
	}
	return &Client{Repo: repo, Token: token, HTTP: &http.Client{Timeout: 0}}
}

func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "dofus-gear-explorer")
	if strings.HasPrefix(url, "https://api.github.com/") {
		req.Header.Set("Accept", "application/vnd.github+json")
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("GitHub refused the request (%s): rate limit reached, try again later or set GITHUB_TOKEN", resp.Status)
		}
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp, nil
}

// Latest returns the most recent release of the repository.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	return c.fetch(ctx, "https://api.github.com/repos/"+c.Repo+"/releases/latest")
}

// ByTag returns one specific release.
func (c *Client) ByTag(ctx context.Context, tag string) (*Release, error) {
	return c.fetch(ctx, "https://api.github.com/repos/"+c.Repo+"/releases/tags/"+tag)
}

func (c *Client) fetch(ctx context.Context, url string) (*Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	if rel.Tag == "" {
		return nil, errors.New("release has no tag")
	}
	return &rel, nil
}

// Download saves an asset to dest, reporting the bytes written so far.
func (c *Client) Download(ctx context.Context, asset Asset, dest string, progress func(written int64)) error {
	if info, err := os.Stat(dest); err == nil && asset.Size > 0 && info.Size() == asset.Size {
		if progress != nil {
			progress(asset.Size)
		}
		return nil
	}
	resp, err := c.get(ctx, asset.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	var written int64
	buf := make([]byte, 256<<10)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				os.Remove(tmp)
				return err
			}
			written += int64(n)
			if progress != nil {
				progress(written)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("download %s: %w", asset.Name, readErr)
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// ExtractIcons unpacks the wanted item icons from an images archive into
// destDir as <iconId>.png. Archive entries are named <iconId>-<size>.png.
func ExtractIcons(archive, destDir string, wanted map[int]bool) (int, error) {
	return ExtractImages(archive, destDir, "", wanted)
}

// ExtractImages unpacks the wanted images of an archive into destDir as
// <id>.png. Archive entries are named <prefix><id>-<size>.png.
func ExtractImages(archive, destDir, prefix string, wanted map[int]bool) (int, error) {
	f, err := os.Open(archive)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer gz.Close()
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return 0, err
	}
	count := 0
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		if hdr.Typeflag != tar.TypeReg || !strings.HasSuffix(hdr.Name, ".png") {
			continue
		}
		// Only the base name is used, so entries cannot escape destDir.
		base := strings.TrimSuffix(path.Base(hdr.Name), ".png")
		if !strings.HasPrefix(base, prefix) {
			continue
		}
		idPart, _, _ := strings.Cut(strings.TrimPrefix(base, prefix), "-")
		id := 0
		if _, err := fmt.Sscanf(idPart, "%d", &id); err != nil || !wanted[id] {
			continue
		}
		out, err := os.Create(filepath.Join(destDir, fmt.Sprintf("%d.png", id)))
		if err != nil {
			return count, err
		}
		_, copyErr := io.Copy(out, io.LimitReader(tr, 8<<20))
		closeErr := out.Close()
		if copyErr != nil {
			return count, copyErr
		}
		if closeErr != nil {
			return count, closeErr
		}
		count++
	}
	return count, nil
}
