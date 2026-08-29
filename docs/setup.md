# Setup

UnpackUI uses the same extraction engine and Starr application integration as
upstream Unpackerr. The quickest way to run this fork is its GHCR image.

## Docker Compose

Create a `compose.yml` file:

```yaml
services:
  unpackui:
    image: ghcr.io/thebadfella/unpackui:1.7.0
    container_name: unpackui
    restart: unless-stopped
    user: "1000:1000"
    ports:
      - "5656:5656"
    volumes:
      - /path/to/downloads:/downloads
      - /path/to/unpackui-config:/config
    environment:
      TZ: America/Regina
      UN_WEBSERVER_UI: "true"
      UN_WEBSERVER_API: "true"
      UN_WEBSERVER_LISTEN_ADDR: 0.0.0.0:5656
      UN_SUPPRESS_MISSING_URLS: "true"
      UN_STATE_FILE: /config/unpackerr.state.json
      UN_MAX_BYTES: 75GB
      UN_MAX_FILES: "5000"
      UN_MAX_RATIO: "15"

      # Add the Starr applications you use.
      UN_SONARR_0_URL: http://sonarr:8989
      UN_SONARR_0_API_KEY: replace-with-your-api-key
      UN_SONARR_0_PATHS_0: /downloads
```

Replace the host paths, user/group IDs, timezone, Starr URL, and API key for
your environment. The `/downloads` path must match the path used by the Starr
application inside its container. Add Radarr, Lidarr, Readarr, Whisparr, or
watched-folder settings as needed using the
[generated Compose reference](../examples/docker-compose.yml).

Start the container:

```console
docker compose up -d
docker compose logs -f unpackui
```

Open `http://localhost:5656`. A startup log line containing `status-ui` confirms
that the UI is enabled.

The versioned tag is best for repeatable deployments. Use `latest` if you want
the newest released image automatically.

## Configuration file

The `/config` mount is optional when all settings are environment variables and
restart recovery does not need persistent storage. If you remove the mount,
remove `UN_STATE_FILE` or point it at another persistent, writable mount. To use
a TOML file, place it at `/path/to/unpackui-config/unpackerr.conf` on the host;
the container discovers it as `/config/unpackerr.conf`.

See [Configuration](configuration.md) for file examples and precedence rules.

## Build from source

Go 1.27 or newer is required for this release.

```console
git clone https://github.com/TheBadFella/UnpackUI.git
cd UnpackUI
git checkout v1.7.0
go build -o unpackerr .
./unpackerr -c ./unpackerr.conf
```

On Windows, use `go build -o unpackerr.exe .` and run
`./unpackerr.exe -c ./unpackerr.conf` from PowerShell.

## Next steps

- Review the [fork-specific environment variables](environment-variables.md).
- Configure the [status UI and JSON API](ui.md).
- Configure [Discord or Notifiarr notifications](notifications.md).
- Use the [upstream documentation](https://unpackerr.zip) for Starr apps,
  watched folders, permissions, archive formats, and other core behavior.
