<div align="center">

<img width="360" alt="Unpackerr" src="https://unpackerr.zip/img/unpackerr.png">

# 📦 UnpackUI

### Unpackerr with the missing pieces filled in

My personal fork of Unpackerr adds a focused status UI, recovery, safer folder
handling, and richer notifications. Feel free to use it if those additions fit
your setup.

[![Release](https://img.shields.io/github/v/release/TheBadFella/UnpackUI?style=for-the-badge&label=release&color=6750A4)](https://github.com/TheBadFella/UnpackUI/releases)
[![Checks](https://img.shields.io/github/actions/workflow/status/TheBadFella/UnpackUI/codetests.yml?branch=web-ui-support&style=for-the-badge&label=checks&color=386A20)](https://github.com/TheBadFella/UnpackUI/actions/workflows/codetests.yml)

<img width="960" alt="UnpackUI status dashboard" src="pkg/ui/UnpackUI.png">

[Upstream repository](https://github.com/Unpackerr/unpackerr) ·
[Official Unpackerr documentation](https://unpackerr.zip)

</div>

## ✨ Highlights

| Feature | What it adds |
|---|---|
| **Live dashboard** | Responsive extraction status, progress, ETA, history, resizable columns, and clear-completed controls. |
| **Restart recovery** | Persists watched-folder work and safely retries interrupted extractions after a restart. |
| **Safer folder handling** | Waits for downloads to finish and ignores media-only or archive-free folders. |
| **Extraction guardrails** | Caps uncompressed bytes, created files, and expansion ratio to protect the disk from rogue archives. |
| **Dashboard API** | Provides aggregate, path-free JSON stats for Homepage and similar tools. |
| **Better notifications** | Adds compact Discord embeds, update-in-place messages, and optional links to the UI. |
| **Quieter optional apps** | Silently skips empty Starr app entries while preserving real configuration and connection errors. |

## 📚 Guides

| Guide | Use it for |
|---|---|
| [Setup](docs/setup.md) | Docker Compose, GHCR images, and source builds |
| [Configuration](docs/configuration.md) | Config files, precedence, recovery, and fork settings |
| [Environment variables](docs/environment-variables.md) | Fork-specific variables and their upstream equivalents |
| [Status UI and API](docs/ui.md) | Dashboard controls, routes, Homepage, and security |
| [Notifications](docs/notifications.md) | Discord, Notifiarr, events, and multiple destinations |

## 🔗 Project

[Releases](https://github.com/TheBadFella/UnpackUI/releases) ·
[Container packages](https://github.com/TheBadFella/UnpackUI/pkgs/container/unpackui) ·
[MIT License](LICENSE)
