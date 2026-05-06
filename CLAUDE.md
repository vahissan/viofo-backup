# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Automated dashcam file sync service for the Viofo A229 Pro. The app runs in Docker, waits for the dashcam to appear on the home network (static IP `10.60.1.8`), fetches the file list via the camera's XML API, and downloads new files to a mounted volume. State is tracked in SQLite so partial syncs survive sudden camera disconnections (car turning off mid-download).

## Module

```
github.com/vahissan/viofo-backup
```

## Commands

```bash
# Build
go build -o dist/viofo-backup ./cmd/viofo-backup

# Run all tests
go test ./...

# Run a single package's tests
go test ./internal/config/

# Run with race detector
go test -race ./...

# Lint (requires golangci-lint)
golangci-lint run

# Build Docker image
docker build -t viofo-backup .

# Run via Docker Compose
docker compose up -d
docker compose logs -f
```

## Architecture

### Package Layout

```
cmd/viofo-backup/    entry point; wires all dependencies, starts sync loop, handles SIGINT/SIGTERM
internal/config/     YAML config loading/validation; custom duration parser (5d / 2m / 1y)
internal/camera/     HTTP client for dashcam APIs: heartbeat check and XML file list fetch
internal/downloader/ streams individual files from camera to a temp path, then renames on success
internal/storage/    manages local directory layout (YYYY-MM-DD/Category/) and disk size queries
internal/tracker/    SQLite-backed record of every successfully downloaded file
internal/retention/  enforces max_age and max_size policies; deletes oldest files first
internal/sync/       orchestrates the poll → sync → wait state machine
internal/logger/     wires lumberjack + slog into a rotating structured logger
```

### State Machine (`internal/sync`)

```
WAITING ──heartbeat OK──► SYNCING ──heartbeat fails──► WAITING
           (poll interval)          (camera went offline)
                                 ↓
                          fetch XML file list
                          for each file not in tracker:
                            download → record in tracker immediately
                          run retention cleanup
                          re-check heartbeat → continue or back to WAITING
```

State transitions are always logged. While `WAITING`, the app logs once on transition to offline and stays silent until state changes again.

### Key Design Decisions

- **Atomic download + track**: files are streamed to a `.tmp` suffix, renamed on completion, then immediately recorded in SQLite. If the camera goes offline mid-file, the incomplete `.tmp` is cleaned up on next startup.
- **SQLite tracker** uses `modernc.org/sqlite` (pure Go, no CGO). On startup, the tracker is reconciled against the local filesystem — files present on disk but missing from the tracker are added so they are not re-downloaded.
- **File layout**: date comes from `<TIME>` in the XML (`YYYY/MM/DD HH:MM:SS`, parse with `"2006/01/02 15:04:05"`). Local paths mirror the camera's subfolder logic: `Movie\RO` and `Movie\` root both land in `/data/YYYY-MM-DD/Movie/`; `Movie\Parking` lands in `/data/YYYY-MM-DD/Movie/Parking/`; `Photo` in `/data/YYYY-MM-DD/Photo/`.
- **Resumable downloads**: the camera server advertises `Accept-Ranges: bytes`. Use HTTP range requests (`Range: bytes=N-`) to resume interrupted downloads instead of restarting from zero.
- **Duration format**: `max_age: "30d"` — the config package implements a custom parser supporting `d` (days), `m` (months), `y` (years) in addition to standard Go duration syntax.
- **Configurable categories**: each config category maps to a specific camera folder — `movie`→`DCIM\Movie\` (root), `parking`→`DCIM\Movie\Parking\`, `emergency`→`DCIM\Movie\RO\`, `photo`→`DCIM\Photo\`. Default: all four. The category filter is evaluated during FPATH parsing, not by top-level folder name.
- **Graceful shutdown**: SIGINT/SIGTERM waits for the current file download to finish before exiting. The tracker is always in a consistent state.
- **No deletes from camera**: the app never modifies or deletes files on the dashcam.

### Camera APIs

Base URL: `http://<camera.ip>`. Both endpoints are unauthenticated, HTTP only (no HTTPS).

#### Heartbeat — `GET /?custom=1&cmd=3016`

```xml
<?xml version="1.0" encoding="UTF-8" ?>
<Function>
  <Cmd>3016</Cmd>
  <Status>0</Status>
</Function>
```

`Status=0` = online. Treat any non-200, connection error, or `Status≠0` as offline.

#### File List — `GET /?custom=1&cmd=3015`

Returns all files in a single response (no pagination). Current scale: ~1,400 files. Response has no `Content-Length` header (connection-close).

```xml
<?xml version="1.0" encoding="UTF-8" ?>
<LIST>
  <ALLFile><File>
    <NAME>2025_1224_120324_000002PF.MP4</NAME>
    <FPATH>A:\DCIM\Movie\RO\2025_1224_120324_000002PF.MP4</FPATH>
    <SIZE>132120576</SIZE>
    <TIMECODE>1536712835</TIMECODE>
    <TIME>2025/12/24 12:04:06</TIME>
    <ATTR>33</ATTR>
  </File></ALLFile>
  ...
</LIST>
```

| Field | Notes |
|---|---|
| `NAME` | Filename only. Format: `YYYY_MMDD_HHMMSS_NNNNNNXX.EXT` |
| `FPATH` | Windows-style full path: `A:\DCIM\<Category>\[SubFolder\]<file>` |
| `SIZE` | Bytes. Matches HTTP `Content-Length` exactly — use for download verification. |
| `TIMECODE` | **Not a Unix timestamp.** Ignore for all date/time logic. |
| `TIME` | `YYYY/MM/DD HH:MM:SS` — the only reliable timestamp. Go parse layout: `"2006/01/02 15:04:05"` |
| `ATTR` | Windows file attributes (32=normal, 33=read-only). Not needed for download logic. |

**Camera folder structure** (real, from live data):

```
DCIM\Movie\RO\       Protected regular recordings (event/emergency clips)
DCIM\Movie\Parking\  Parking mode recordings
DCIM\Movie\          Unprotected regular recordings
DCIM\Photo\          Photo captures (.JPG)
```

Categories map to camera folders as follows:

| Config category | Camera folder | Local path |
|---|---|---|
| `movie` | `DCIM\Movie\` (root) | `YYYY-MM-DD/Movie/` |
| `parking` | `DCIM\Movie\Parking\` | `YYYY-MM-DD/Movie/Parking/` |
| `emergency` | `DCIM\Movie\RO\` | `YYYY-MM-DD/Movie/Emergency/` |
| `photo` | `DCIM\Photo\` | `YYYY-MM-DD/Photo/` |

#### File Download

Convert FPATH to URL: strip `A:\`, replace `\` with `/`, prepend `http://<ip>/`.

```
FPATH: A:\DCIM\Photo\2026_0505_175704_047965PF.JPG
URL:   http://10.60.1.8/DCIM/Photo/2026_0505_175704_047965PF.JPG
```

Server responds with `Accept-Ranges: bytes` — use range requests to resume interrupted downloads.

### Config Reference (`config.yaml`)

```yaml
camera:
  ip: "10.60.1.8"
  heartbeat_interval: "5m"     # how often to poll while waiting
  categories:                  # omit to download all; valid: movie, parking, emergency, photo
    - movie
    - parking
    - emergency
    - photo

download:
  directory: "/data"           # docker volume mount point

retention:
  max_age: "30d"               # d=days, m=months, y=years; delete files older than this
  max_size: "50GB"             # KB/MB/GB/TB; delete oldest files first when exceeded

logging:
  file: "/logs/viofo-backup.log"
  max_size_mb: 100             # rotate when file reaches this size
  max_backups: 5               # number of rotated files to keep
  max_age_days: 30             # delete rotated files older than this
  compress: true               # gzip rotated files
```

### Docker Layout

```
/data/     → mounted volume for downloaded dashcam files
/logs/     → mounted volume for log files
/app/config.yaml → bind-mounted config file
```

The container runs as a non-root user. The `Dockerfile` uses a multi-stage build: `golang:alpine` builder → `alpine` runtime image.

### Dependencies

| Package | Purpose |
|---|---|
| `gopkg.in/yaml.v3` | Config parsing |
| `modernc.org/sqlite` | Pure-Go SQLite (no CGO required) |
| `gopkg.in/lumberjack.v2` | Log file rotation (used as `slog` writer) |
| Standard library | HTTP client, XML parsing, filesystem ops |
