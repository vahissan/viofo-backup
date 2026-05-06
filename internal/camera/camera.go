package camera

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client is an HTTP client for the Viofo dashcam APIs.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(ip string) *Client {
	return &Client{
		baseURL:    "http://" + ip,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// IsOnline returns true when the camera is reachable and returns Status=0.
// The check uses a 5-second timeout appropriate for a local network.
func (c *Client) IsOnline(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/?custom=1&cmd=3016", nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return false
	}
	defer resp.Body.Close()

	var result struct {
		XMLName xml.Name `xml:"Function"`
		Status  int      `xml:"Status"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	return result.Status == 0
}

// File is a single downloadable entry from the camera.
type File struct {
	Name        string
	FPATH       string
	Size        int64
	Time        time.Time
	Category    string // "movie", "parking", "emergency", "photo"
	LocalSubdir string // e.g. "Movie", "Movie/Parking", "Movie/Emergency", "Photo"
	DownloadURL string
}

type fileXML struct {
	Name    string `xml:"NAME"`
	FPATH   string `xml:"FPATH"`
	Size    int64  `xml:"SIZE"`
	TimeStr string `xml:"TIME"`
}

type listXML struct {
	Files []fileXML `xml:"ALLFile>File"`
}

// ListFiles fetches and parses the complete file list from the camera.
func (c *Client) ListFiles() ([]File, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/?custom=1&cmd=3015")
	if err != nil {
		return nil, fmt.Errorf("fetch file list: %w", err)
	}
	defer resp.Body.Close()

	var list listXML
	if err := xml.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode file list: %w", err)
	}

	files := make([]File, 0, len(list.Files))
	for _, f := range list.Files {
		t, err := time.ParseInLocation("2006/01/02 15:04:05", f.TimeStr, time.Local)
		if err != nil {
			continue
		}
		cat, localSubdir := classifyPath(f.FPATH)
		files = append(files, File{
			Name:        f.Name,
			FPATH:       f.FPATH,
			Size:        f.Size,
			Time:        t,
			Category:    cat,
			LocalSubdir: localSubdir,
			DownloadURL: fpathToURL(c.baseURL, f.FPATH),
		})
	}
	return files, nil
}

// classifyPath maps a Windows-style camera FPATH to a logical category and local subdirectory.
//
//	DCIM\Movie\RO\      → "emergency", "Movie/Emergency"
//	DCIM\Movie\Parking\ → "parking",   "Movie/Parking"
//	DCIM\Movie\         → "movie",     "Movie"
//	DCIM\Photo\         → "photo",     "Photo"
func classifyPath(fpath string) (category, localSubdir string) {
	upper := strings.ToUpper(fpath)
	switch {
	case strings.Contains(upper, `\MOVIE\RO\`):
		return "emergency", "Movie/Emergency"
	case strings.Contains(upper, `\MOVIE\PARKING\`):
		return "parking", "Movie/Parking"
	case strings.Contains(upper, `\MOVIE\`):
		return "movie", "Movie"
	case strings.Contains(upper, `\PHOTO\`):
		return "photo", "Photo"
	default:
		return "unknown", "Other"
	}
}

// fpathToURL converts "A:\DCIM\Photo\file.JPG" → "http://ip/DCIM/Photo/file.JPG".
func fpathToURL(baseURL, fpath string) string {
	if len(fpath) >= 3 && fpath[1] == ':' {
		fpath = fpath[3:]
	}
	return baseURL + "/" + strings.ReplaceAll(fpath, `\`, "/")
}
