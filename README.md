# BusyGraph

BusyGraph is a background application for macOS and Linux that tracks your keystrokes and mouse activity (distance, clicks, scrolls), aggregating them by minute. It exports these metrics via a built-in Prometheus exporter and provides a local dashboard for visualization.

![BusyGraph Dashboard](screenshot.png)

## Features

-   **Keystroke Tracking**: Counts total keystrokes per minute.
-   **Mouse Tracking**: Tracks mouse distance (pixels), clicks (left/right), and scroll usage.
-   **Privacy-Focused**: Data is stored locally on your machine in a SQLite database.
-   **Dashboard**: Built-in web dashboard to view your activity stats over time (24h, 7d, 30d, 1y).
-   **Prometheus Metrics**: Exposes standard Prometheus metrics at `/metrics` for integration with your own monitoring stack (Grafana, etc.).
-   **System Tray**: Runs quietly in the background with a system tray icon for quick control.

## Installation

### Prerequisites

-   **Go**: 1.21 or later.
-   **Linux (Debian/Ubuntu)**:
    ```bash
    sudo apt-get install libwebkit2gtk-4.0-dev libgtk-3-dev libayatana-appindicator3-dev
    ```
-   **Linux (Arch)**:
    ```bash
    sudo pacman -S webkit2gtk-4.1 libayatana-appindicator gtk3
    # webview_go expects webkit2gtk-4.0 pkg-config; create a symlink for the compatible 4.1 package
    sudo ln -s /usr/lib/pkgconfig/webkit2gtk-4.1.pc /usr/lib/pkgconfig/webkit2gtk-4.0.pc
    ```

### Building from Source

```bash
git clone https://github.com/victortrac/busygraph.git
cd busygraph
go build -o busygraph .
```

## Usage

Run the application:

```bash
./busygraph
```

Or run directly with Go:

```bash
go run .
```

On **macOS**, you will be prompted to grant "Accessibility" permissions on the first run. This is required to listen to global input events.

On **Linux**, your user must be in the `input` group to read from `/dev/input/event*` devices:
```bash
sudo usermod -aG input $USER
```
Then log out and back in for the change to take effect.

### Dashboard

Click "Open Dashboard" in the system tray menu, or navigate to:
[http://localhost:2112/dashboard](http://localhost:2112/dashboard)

### Metrics

Prometheus metrics are available at:
[http://localhost:2112/metrics](http://localhost:2112/metrics)

## Data Location

BusyGraph stores its data in a SQLite database located at:

-   **Linux/macOS**: `$XDG_DATA_HOME/busygraph/<hostname>.db` (usually `~/.local/share/busygraph/<hostname>.db`)

On first run, if a legacy `busygraph.db` exists it is automatically renamed to `<hostname>.db`.

### Multi-Machine Federation

If you use BusyGraph on multiple machines, you can consolidate all their data into a single dashboard view. Each machine names its database after its hostname (e.g. `laptop.db`, `desktop.db`). Sync the data directory between machines using [Syncthing](https://syncthing.net/), rsync, or any file sync tool so that all `*.db` files end up in the same directory on each machine.

BusyGraph automatically discovers and attaches any peer `*.db` files it finds in the data directory (every 30 seconds). Read queries (dashboard, heatmaps, stats) combine data from all attached databases via `UNION ALL` views. Writes always go to the local machine's own database only, so there are no conflicts.

**Setup:**

1. Install BusyGraph on each machine and run it once so it creates `<hostname>.db`.
2. Configure Syncthing (or similar) to sync `~/.local/share/busygraph/` across your machines.
3. That's it — the dashboard will show combined activity from all machines within 30 seconds of a new DB file appearing.

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## License

[MIT](LICENSE)
