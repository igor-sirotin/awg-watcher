# AWG Watcher

Local Keenetic/Entware web app that watches Amnezia Premium country config metadata and sends Telegram notifications when it changes. V1 is read-only: it calls only `v1/account_info` and does not modify AWG-Manager or router tunnel configs.

## Current Status

Implemented:

- single Go binary with embedded React/shadcn-style web UI
- `frontend/` Vite React source tree with built assets embedded from `internal/watch/static`
- `vpn://` decoder for compressed and plain exported keys
- multiple AmneziaVPN keys with independent country watch lists
- local config/state files with `0600` writes
- password-protected web UI after first setup
- fixture mode for offline tests and local UI simulation
- read-only Amnezia `v1/account_info` client
- country metadata diffing for `worker_last_updated` and `last_downloaded`
- Telegram notifications and test notification
- redacted diagnostics export
- OPKG package/feed scaffold for Keenetic/Entware

The production Amnezia gateway public key is not committed. To run live gateway calls, put it in a local PEM file and pass `--gateway-pk-filepath`, or use the default key-file path described below.

## Build And Test

```sh
make test
make build
```

The binary is written to:

```text
bin/awg-watcher
```

`make build` runs the frontend build first and then compiles Go. For frontend-only
development:

```sh
cd frontend
npm install
npm run dev
```

The development server serves only the UI source. Run the Go server separately for
the API:

```sh
go run ./cmd/awg-watcher serve --workdir ./local-data
```

For releases, use `make opkg-feed VERSION=...` so the current frontend is embedded
into the Entware binary and packaged as an `.ipk`.

## Offline Local Run

Fixture mode does not call Amnezia or Telegram unless you press the Telegram test button.

```sh
go run ./cmd/awg-watcher serve \
  --workdir ./local-data \
  --fixture-account-info ./testdata/account_info_baseline.json
```

On first run the server prints a setup token and URL:

```text
http://127.0.0.1:8097/?setup_token=...
```

Open that URL, set an admin password, configure countries such as `EE, NL`, save, then click `Check now`.

The UI uses a full-screen sidebar layout:

- `Dashboard`: overall status, schedule, key summary, and recent key status.
- `Keys`: add AmneziaVPN keys, run checks, and choose countries from each key's account response.
- `AWG Manager`: placeholder only; no router tunnel changes are made in V1.
- `Tools`: Telegram test and redacted diagnostics download.
- `Settings`: popup for password, gateway public keys, gateway endpoint, poll interval, and Telegram.

For a new key, save the key first. The UI runs a check, loads available countries, then lets you select which countries to watch.

To simulate a change, restart with:

```sh
go run ./cmd/awg-watcher serve \
  --workdir ./local-data \
  --fixture-account-info ./testdata/account_info_changed.json
```

Click `Check now` again; Estonia should report changed metadata.

## CLI

```sh
awg-watcher serve
awg-watcher check
awg-watcher decode < key.txt
awg-watcher notify-test
awg-watcher status
```

Useful local flags:

```sh
--workdir ./local-data
--config ./config.json
--state ./state.json
--gateway-pk-filepath ./gateway_public_key.pem
--fixture-account-info ./testdata/account_info_baseline.json
```

`--workdir` stores `config.json`, `state.json`, and the default gateway public key file
inside that directory. It is the easiest way to run locally without writing to `/opt`.

`decode` prints redacted JSON by default. Use `--show-secrets` only on a trusted machine.

## Gateway Public Key File

### Extract from AmneziaVPN on macOS

The Amnezia gateway public key is embedded in the official AmneziaVPN app. On macOS,
extract public PEM blocks from the installed app with:

```sh
strings -a /Applications/AmneziaVPN.app/Contents/MacOS/AmneziaVPN \
  | awk '/-----BEGIN PUBLIC KEY-----/{p=1} p{print} /-----END PUBLIC KEY-----/{p=0; print ""}' \
  > gateway_public_key.pem
```

The app binary may contain more than one public key. Keep all extracted public-key
PEM blocks in the file; the watcher tries them in order and uses the first one that
works for the read-only `v1/account_info` request.

The key file should look like this:

```text
-----BEGIN PUBLIC KEY-----
...
-----END PUBLIC KEY-----

-----BEGIN PUBLIC KEY-----
...
-----END PUBLIC KEY-----
```

Keep this file local. It is a public key, but it is still an implementation detail
from the Amnezia app and should not be committed.

### Local path

The default gateway public key path on Entware is:

```text
/opt/etc/awg-watcher/gateway_public_key.pem
```

When running locally with `--workdir ./local-data`, the default becomes:

```text
./local-data/gateway_public_key.pem
```

Create the file with mode `0600`:

```sh
mkdir -p ./local-data
cp gateway_public_key.pem ./local-data/gateway_public_key.pem
chmod 600 ./local-data/gateway_public_key.pem
```

Then run:

```sh
go run ./cmd/awg-watcher serve --workdir ./local-data
```

To use a different key file:

```sh
go run ./cmd/awg-watcher check \
  --workdir ./local-data \
  --gateway-pk-filepath ./private/gateway_public_key.pem
```

### Entware path

On the router, install the key file at the default path:

```sh
mkdir -p /opt/etc/awg-watcher
cp gateway_public_key.pem /opt/etc/awg-watcher/gateway_public_key.pem
chmod 600 /opt/etc/awg-watcher/gateway_public_key.pem
```

The init script can then run without an explicit `--gateway-pk-filepath` argument.

## Keenetic/Entware Usage

Default paths:

```text
/opt/etc/awg-watcher/config.json
/opt/var/lib/awg-watcher/state.json
```

Default listener:

```text
127.0.0.1:8097
```

Recommended access from your workstation:

```sh
ssh -L 8097:127.0.0.1:8097 root@router.example
```

Then open:

```text
http://127.0.0.1:8097/
```

Publish the hosted OPKG feed:

1. Commit and push the workflow and packaging files to the default branch.
2. In GitHub, open `Settings` -> `Pages` and set the source to `GitHub Actions`.
   If Pages is not enabled, deployment fails with `Failed to create deployment
   (status: 404) ... Ensure GitHub Pages has been enabled`.
3. Create and push a version tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The `OPKG Release` workflow builds the package, attaches the `.ipk` to the GitHub
release, and deploys the feed to GitHub Pages. After it succeeds, verify:

```text
https://igor-sirotin.github.io/awg-watcher/opkg/mipselsf-k3.4/
https://igor-sirotin.github.io/awg-watcher/opkg/mipselsf-k3.4/Packages.gz
```

You can also publish without creating a tag by running `OPKG Release` manually
from the GitHub Actions tab and entering a version. Manual runs update GitHub
Pages but do not create a GitHub release.

Install from the hosted OPKG feed after the workflow has deployed:

```sh
echo 'src/gz awg-watcher https://igor-sirotin.github.io/awg-watcher/opkg/mipselsf-k3.4' > /opt/etc/opkg/awg-watcher.conf
opkg update
opkg install awg-watcher
/opt/etc/init.d/S97awg-watcher start
```

Upgrade later with:

```sh
opkg update
opkg upgrade awg-watcher
```

Build a local package and OPKG feed:

```sh
make opkg-feed VERSION=0.1.0
```

The generated files are written to:

```text
dist/opkg/mipselsf-k3.4/
```

The package installs the binary and init script, then creates the config and
state directories if they do not already exist. It does not include or overwrite
local config, state, or gateway key files.

To debug a package before publishing, inspect and copy the local `.ipk`:

```sh
tar -tzf dist/opkg/mipselsf-k3.4/awg-watcher_0.1.0_mipsel-3.4.ipk
tar -xOf dist/opkg/mipselsf-k3.4/awg-watcher_0.1.0_mipsel-3.4.ipk ./control.tar.gz | tar -tzf -
tar -xOf dist/opkg/mipselsf-k3.4/awg-watcher_0.1.0_mipsel-3.4.ipk ./data.tar.gz | tar -tzf -
scp dist/opkg/mipselsf-k3.4/awg-watcher_0.1.0_mipsel-3.4.ipk root@router.example:/tmp/
ssh root@router.example 'opkg install -V3 /tmp/awg-watcher_0.1.0_mipsel-3.4.ipk'
```

The archive listing must not contain `._*` AppleDouble files or `PaxHeader`
entries; Entware `opkg` cannot extract those.

## Live Amnezia Account Check

Configure:

- Amnezia Premium `vpn://` key
- countries to watch
- gateway public key file via `--gateway-pk-filepath`
- Telegram bot token and chat ID, if notifications are wanted

The live client sends one encrypted POST to:

```text
<gateway_endpoint>/v1/account_info
```

It does not call `v1/config`, `v1/native_config`, revoke endpoints, or AWG-Manager endpoints.
