# Scene Palette

Turn an image into a readable colour palette for interfaces, websites, and Godot projects.
Drop in a photograph, illustration, or screenshot, then preview and export the result.

Scene Palette runs locally. Use the standalone HTML file with **no installation**, or run
the Go app for a local browser interface and command-line tools. The two engines are
tested against each other, and both use the same palette file format.

## Quick start — no installation

1. On this repository's GitHub page, choose **Code → Download ZIP**.
2. Extract the ZIP, then open **`index.html`** in a current desktop browser.
3. Drop an image onto the image area and click **Forge plate**.

That's it. The standalone app works offline and does not need Go, Node.js, or a web server.
Images are processed on your device.

## Make your first palette

1. Add one image, or up to eight images to combine their colours. PNG and JPEG are good starting points.
2. Leave polarity on **Auto** and click **Forge plate**. A *plate* is a palette with eight named roles:
   `ground`, `panel`, `ink`, `muted`, `observed`, `attention`, `rupture`, and `gold`.
3. Check the previews and **Readability** matrix. Try **Dark** or **Light** to change the
   background style, or **Remix** to try another accent combination when one is available.
4. To keep a particular colour, use a role's **pick** control and click the image. Picking locks
   that role through later extractions; unlock it to let the app change it again.
5. Click **Save** to keep the plate on the shelf. Choose an **Export** format, then **Download**
   to save a file you can use elsewhere.

Use **Export shelf** to back up your saved plates. Browser storage can be cleared or unavailable,
especially when opening local files; keep an exported copy of work you want to retain.

## Export formats

| Format | Use it for |
| --- | --- |
| **CSS** | Custom properties for a website or interface. |
| **Godot 4 pack** | A GDScript colour resource for a Godot project. |
| **JSON** | Reopening a palette with its roles, metadata, and colour origins intact. |
| **GPL** | A palette for compatible art tools such as GIMP, Aseprite, or Krita. |
| **HEX** | A plain list of colour values. |
| **Bundle** | A ZIP with the palette, exports, swatch preview, and available source images. |

You can also compare saved palettes, pair light and dark themes, or export a whole shelf as a
vault. The [user guide](docs/USER_GUIDE.md) covers these workflows and every control.

## Windows app

If you have a portable Windows ZIP, extract it and double-click **`scene-palette.exe`** or
**`Run Scene Palette.cmd`**. It opens the app in your default browser. Keep its console window
open while you work; close it to stop the local server. No Go installation is needed to run
the packaged executable. The included `index.html` also works on its own.

**GitHub's Code → Download ZIP contains source files, not a Windows executable.** Use the
no-install HTML instructions above, or build a portable Windows x64 ZIP from source below.

### Build the Windows ZIP

With **Go 1.23 or newer** and **Git** on your `PATH`, run these commands in PowerShell:

```powershell
git clone https://github.com/mythicruntime-lab/scene-palette.git
cd scene-palette
powershell -NoProfile -File scripts/package-windows.ps1
```

The ZIP and its checksums appear in `bin/releases/`. The package includes the executable,
standalone HTML, launcher, usage notes, licenses, and build information. Builds are unsigned.

## Run from source

With **Go 1.23 or newer** installed, open a terminal in the extracted or cloned repository:

```sh
go run . serve -open
```

The app opens at a loopback address, usually `http://127.0.0.1:8765/`. Keep the terminal open
while using it; press **Ctrl+C** to stop. The default launcher chooses another port if 8765
is busy. There is no frontend build step or npm install required to run the app.

For command-line extraction, pass a PNG, JPEG, or GIF file:

```sh
go run . forge -out palette.json -name "Night bay" "path/to/image.png"
```

See the [command reference](docs/USER_GUIDE.md#command-line-reference) for bundles, forced
polarity, and palette verification.

## What to check in a result

Generated body text targets **4.5:1 contrast** on both background surfaces; muted text and
accents target **3:1**. These checks help you choose a palette, but the readability of a
finished interface also depends on text size, layout, and how you apply the colours.

Picked and locked colours stay exactly as chosen, even when they fail contrast checks.
Raw **found inks** also preserve source colours. Inspect the Readability matrix before use.
Forcing a light image into a dark palette, or the reverse, can change colours substantially.
Colour origins record adjustments, and **derived** badges mark colours the app had to create.

## Documentation and development

| Document | Contents |
| --- | --- |
| [User guide](docs/USER_GUIDE.md) | Controls, extraction, themes, shelves, bundles, and the CLI. |
| [Contributing](CONTRIBUTING.md) | Development setup and Go, JavaScript, parity, and browser tests. |
| [Quality report](docs/QUALITY_REPORT.md) | Test results, coverage, and known limitations. |
| [Go architecture](docs/GO_MIGRATION.md) | Engine boundaries, local API, and durable file contract. |
| [Security](SECURITY.md) | Privacy, vulnerability reporting, and secret scanning. |

The repository includes ten generated test patterns. Regression checks cover exact Go/HTML
palette parity and browser workflows. The quality report separates those reproducible checks
from the earlier real-image evaluation; synthetic patterns alone do not establish visual quality.

## License

Code, documentation, and generated test patterns are licensed under the [MIT License](LICENSE).
Bundled and adapted components retain the notices in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
Images you supply remain subject to their owners' terms.
