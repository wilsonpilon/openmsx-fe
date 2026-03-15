// Package filehunter provides a client for browsing download.file-hunter.com
// which is a plain Apache directory listing of MSX software.
package filehunter

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

const BaseURL = "https://download.file-hunter.com/"

// Entry represents a file or directory in the FileHunter archive.
type Entry struct {
	Name     string
	URL      string
	IsDir    bool
	Size     string
	Modified string
	FileType string // rom, dsk, cas, etc.
}

// Client is a FileHunter HTTP browser.
type Client struct {
	http    *http.Client
	baseURL string
}

// New creates a new FileHunter client.
func New() *Client {
	return &Client{
		http:    &http.Client{Timeout: 15 * time.Second},
		baseURL: BaseURL,
	}
}

// List fetches a directory listing from the given path.
// dirPath may be empty (root), a relative path, or a full absolute URL.
func (c *Client) List(dirPath string) ([]Entry, error) {
	var u string
	switch {
	case dirPath == "" || dirPath == "/":
		u = c.baseURL
	case strings.HasPrefix(dirPath, "http://") || strings.HasPrefix(dirPath, "https://"):
		// Already an absolute URL (e.g. from navigating into a subdirectory)
		u = dirPath
		if !strings.HasSuffix(u, "/") {
			u += "/"
		}
	default:
		// Relative path: join with base, avoiding double slashes
		rel := strings.TrimPrefix(dirPath, "/")
		u = strings.TrimSuffix(c.baseURL, "/") + "/" + rel
		if !strings.HasSuffix(u, "/") {
			u += "/"
		}
	}

	resp, err := c.http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, u)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return parseDirectoryListing(string(body), u), nil
}

// Search filters entries matching the query string (case-insensitive).
func (c *Client) Search(dirPath, query string) ([]Entry, error) {
	entries, err := c.List(dirPath)
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(query)
	var results []Entry
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Name), q) {
			results = append(results, e)
		}
	}
	return results, nil
}

// Download fetches a file from fileURL and returns its content.
func (c *Client) Download(fileURL string) ([]byte, error) {
	resp, err := c.http.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", fileURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return data, nil
}

// ── Parsing ──────────────────────────────────────────────────────────────────

// Apache-style directory listing parser.
// Matches lines like:
//   <a href="Games/">Games/</a>   2023-01-15 10:00    -
//   <a href="Gradius.rom">Gradius.rom</a>  2023-01-15  512K

var (
	reLink = regexp.MustCompile(`href="([^"]+)"`)
	reRow  = regexp.MustCompile(`<a\s+href="([^"?][^"]*)"[^>]*>([^<]+)</a>\s*([\d-]+\s[\d:]+|-)\s+(\S+|-)`)
)

func parseDirectoryListing(body, baseURL string) []Entry {
	var entries []Entry

	lines := strings.Split(body, "\n")
	for _, line := range lines {
		m := reRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		href := m[1]
		name := strings.TrimSuffix(m[2], "/")
		modified := strings.TrimSpace(m[3])
		size := strings.TrimSpace(m[4])

		// Skip parent directory link
		if href == "../" || href == "./" || strings.HasPrefix(href, "?") {
			continue
		}

		isDir := strings.HasSuffix(href, "/")

		// Build absolute URL
		absURL := href
		if !strings.HasPrefix(href, "http") {
			base, err := url.Parse(baseURL)
			if err == nil {
				ref, err := url.Parse(href)
				if err == nil {
					absURL = base.ResolveReference(ref).String()
				}
			}
		}

		ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))

		entries = append(entries, Entry{
			Name:     name,
			URL:      absURL,
			IsDir:    isDir,
			Size:     size,
			Modified: modified,
			FileType: ext,
		})
	}

	// Fallback: just extract href links
	if len(entries) == 0 {
		matches := reLink.FindAllStringSubmatch(body, -1)
		for _, m := range matches {
			href := m[1]
			if href == "../" || href == "./" || strings.HasPrefix(href, "?") || strings.HasPrefix(href, "http") {
				continue
			}
			name := strings.TrimSuffix(href, "/")
			isDir := strings.HasSuffix(href, "/")
			absURL := href
			base, err := url.Parse(baseURL)
			if err == nil {
				ref, err := url.Parse(href)
				if err == nil {
					absURL = base.ResolveReference(ref).String()
				}
			}
			ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
			entries = append(entries, Entry{
				Name:     name,
				URL:      absURL,
				IsDir:    isDir,
				FileType: ext,
			})
		}
	}

	return entries
}

// IsMediaFile returns true if the file can be loaded into openMSX.
func IsMediaFile(ext string) bool {
	switch ext {
	case "rom", "dsk", "cas", "zip", "gz", "mx1", "mx2":
		return true
	}
	return false
}

// MediaType returns a human-readable media type.
func MediaType(ext string) string {
	switch ext {
	case "rom", "mx1", "mx2":
		return "ROM"
	case "dsk":
		return "Disk"
	case "cas":
		return "Cassette"
	case "zip":
		return "Archive"
	case "vgm", "vgz":
		return "Music"
	default:
		if ext == "" {
			return "Dir"
		}
		return strings.ToUpper(ext)
	}
}
