# Amnezia Config Watcher

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
- Entware init-script scaffold

The production Amnezia gateway public key is not committed. To run live gateway calls, put it in a local PEM file and pass `--gateway-pk-filepath`, or use the default key-file path described below.

## Build And Test

```sh
go test ./...
make build
```

The binary is written to:

```text
bin/amnezia-config-watch
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
go run ./cmd/amnezia-config-watch serve --workdir ./local-data
```

For releases, use `make build` or `make build-linux` so the current frontend is
embedded into the binary.

## Offline Local Run

Fixture mode does not call Amnezia or Telegram unless you press the Telegram test button.

```sh
go run ./cmd/amnezia-config-watch serve \
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
go run ./cmd/amnezia-config-watch serve \
  --workdir ./local-data \
  --fixture-account-info ./testdata/account_info_changed.json
```

Click `Check now` again; Estonia should report changed metadata.

## CLI

```sh
amnezia-config-watch serve
amnezia-config-watch check
amnezia-config-watch decode < key.txt
amnezia-config-watch notify-test
amnezia-config-watch status
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
/opt/etc/amnezia-config-watch/gateway_public_key.pem
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
go run ./cmd/amnezia-config-watch serve --workdir ./local-data
```

To use a different key file:

```sh
go run ./cmd/amnezia-config-watch check \
  --workdir ./local-data \
  --gateway-pk-filepath ./private/gateway_public_key.pem
```

### Entware path

On the router, install the key file at the default path:

```sh
mkdir -p /opt/etc/amnezia-config-watch
cp gateway_public_key.pem /opt/etc/amnezia-config-watch/gateway_public_key.pem
chmod 600 /opt/etc/amnezia-config-watch/gateway_public_key.pem
```

The init script can then run without an explicit `--gateway-pk-filepath` argument.

## Keenetic/Entware Usage

Default paths:

```text
/opt/etc/amnezia-config-watch/config.json
/opt/var/lib/amnezia-config-watch/state.json
```

Default listener:

```text
127.0.0.1:8097
```

Recommended access from your workstation:

```sh
ssh -L 8097:127.0.0.1:8097 keenetic
```

Then open:

```text
http://127.0.0.1:8097/
```

Build for a common Keenetic/Entware MIPS little-endian target:

```sh
make build-linux
```

Install manually until a complete OPKG recipe is added:

```sh
install -m 0755 bin/amnezia-config-watch-mipsle /opt/bin/amnezia-config-watch
install -m 0755 packaging/entware/S97amnezia-config-watch /opt/etc/init.d/S97amnezia-config-watch
mkdir -p /opt/etc/amnezia-config-watch /opt/var/lib/amnezia-config-watch
chmod 700 /opt/etc/amnezia-config-watch /opt/var/lib/amnezia-config-watch
/opt/etc/init.d/S97amnezia-config-watch start
```

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
