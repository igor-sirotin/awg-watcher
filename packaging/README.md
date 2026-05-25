# OPKG Packaging Notes

The package should install:

- `/opt/bin/awg-watcher`
- `/opt/etc/init.d/S97awg-watcher`
- `/opt/etc/awg-watcher/`
- `/opt/var/lib/awg-watcher/`

Build a Keenetic/Entware-oriented binary from macOS with:

```sh
make build-entware-mipsel
```

Build an `.ipk` and local feed metadata with:

```sh
make opkg-feed VERSION=0.1.0
```

The generated feed is written to:

```text
dist/opkg/mipselsf-k3.4/
```

It contains:

- `awg-watcher_<version>_mipsel-3.4.ipk`
- `Packages`
- `Packages.gz`
- `index.html` in the feed directory
- `index.html` at the site root for GitHub Pages

The package creates `/opt/etc/awg-watcher` and `/opt/var/lib/awg-watcher`
during installation, but it does not package local config, state, or gateway key
files.

Entware `.ipk` files are gzip-compressed tar archives containing
`debian-binary`, `control.tar.gz`, and `data.tar.gz`. Inspect a local build with:

```sh
tar -tzf dist/opkg/mipselsf-k3.4/awg-watcher_0.1.0_mipsel-3.4.ipk
tar -xOf dist/opkg/mipselsf-k3.4/awg-watcher_0.1.0_mipsel-3.4.ipk ./control.tar.gz | tar -tzf -
tar -xOf dist/opkg/mipselsf-k3.4/awg-watcher_0.1.0_mipsel-3.4.ipk ./data.tar.gz | tar -tzf -
```
