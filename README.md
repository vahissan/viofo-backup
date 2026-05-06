# viofo-backup

Automatically syncs footage from a **Viofo A229 Pro** dashcam to local storage whenever it connects to your home network via WiFi Station Mode.

- Polls the camera's heartbeat endpoint and begins syncing the moment it comes online
- Resumes interrupted downloads (HTTP range requests)
- Tracks downloads in SQLite — survives sudden disconnections (car turning off)
- Configurable retention by age and total disk size
- Rotating log files

## Acknowledgements

This project was inspired by a [Reddit thread](https://www.reddit.com/r/VIOFO/comments/1kdcjsf/wifi_station_mode_sync_at_home/) in r/VIOFO that demonstrated using WiFi Station Mode for home network sync, and by [RobXYZ/viofosync](https://github.com/RobXYZ/viofosync), which pioneered the approach of talking to the dashcam's HTTP API from a home server.

## Quick Start

### Docker

```bash
docker run -d \
  --name viofo-backup \
  --restart unless-stopped \
  -v /path/to/config.yaml:/app/config.yaml:ro \
  -v /path/to/data:/data \
  -v /path/to/logs:/logs \
  -e TZ=America/Vancouver \
  vahissan/viofo-backup:latest
```

If your data directory is owned by a specific user, add `-u UID:GID` to match it and avoid permission errors.

### Docker Compose

```yaml
services:
  viofo-backup:
    image: vahissan/viofo-backup:latest
    user: "1000:1000"               # optional: set to UID:GID that owns your data directory
    volumes:
      - /path/to/config.yaml:/app/config.yaml:ro
      - dashcam-data:/data
      - dashcam-logs:/logs
    environment:
      - TZ=America/Vancouver        # optional: log timestamps in local time
    restart: unless-stopped

volumes:
  dashcam-data:
  dashcam-logs:
```

## Configuration

Create a `config.yaml` from the example below. The only required field is `camera.ip`.

```yaml
camera:
  ip: "192.168.1.100"             # IP assigned to your dashcam on your network
  heartbeat_interval: "5m"       # how often to poll when camera is offline
  categories:                    # omit to sync all categories
    - movie                      # DCIM\Movie\         regular clips
    - parking                    # DCIM\Movie\Parking\ parking mode
    - emergency                  # DCIM\Movie\RO\      event/protected clips
    - photo                      # DCIM\Photo\

download:
  directory: "/data"

retention:
  max_age: "30d"                 # d = days, m = months, y = years
  max_size: "50GB"               # KB / MB / GB / TB

logging:
  file: "/logs/viofo-backup.log"
  max_size_mb: 100
  max_backups: 5
  max_age_days: 30
  compress: true
```

## File Layout

Downloads are organised by date and category:

```
/data/
  2024-01-15/
    Movie/
    Movie/Parking/
    Movie/Emergency/
    Photo/
```

The SQLite tracker database is stored at `/data/.viofo-backup.db`.

## Building from Source

```bash
git clone https://github.com/vahissan/viofo-backup
cd viofo-backup
go build -o dist/viofo-backup ./cmd/viofo-backup
```

Requires Go 1.25+. No CGO — the binary is fully static.

### Multi-platform Docker image (amd64 + arm64)

```bash
docker buildx create --use --name multibuilder --driver docker-container
docker buildx build --platform linux/amd64,linux/arm64 -t vahissan/viofo-backup:latest --push .
```
