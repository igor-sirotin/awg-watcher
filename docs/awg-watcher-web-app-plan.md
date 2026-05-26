# AWG Watcher Web App Plan

This document records the implementation plan and discovered technical details for a v1 Keenetic/Entware web app that detects Amnezia Premium configuration changes.

## Goal

Build a local Entware web app for Keenetic that:

- Accepts an Amnezia Premium `vpn://` key.
- Fetches Amnezia account/config metadata programmatically.
- Lets the user select countries to monitor.
- Detects when Amnezia Premium configuration files for those countries change.
- Shows status in a local Web UI.
- Sends Telegram notifications when changes are detected.

V1 is detection and notification only. It must not replace AWG-Manager tunnel configs automatically.

## Target Deployment

- Router: Keenetic device with Entware support.
- Runtime: Entware on Keenetic.
- Packaging: OPKG package.
- Implementation: single Go binary with embedded HTML/CSS/JS.
- Default listen address: `127.0.0.1:8097`.
- LAN binding should be explicit, not the default.
- Recommended access during testing:

```sh
ssh -L 8097:127.0.0.1:8097 root@router.example
```

Then open:

```text
http://127.0.0.1:8097/
```

## V1 Web App Scope

### First-Run Setup

- Paste `vpn://` key.
- Decode/test the key locally.
- Fetch Amnezia account info.
- Show available countries from the account response.
- Let user select countries to monitor.
- Configure Telegram bot token and chat ID.
- Send a Telegram test notification.

### Dashboard

- Show watched countries.
- Show last check time.
- Show last known `worker_last_updated` and `last_downloaded` values.
- Show current status: OK, changed, API/subscription error, Telegram error, or unknown.
- Provide a manual "Check now" action.

### Settings

- Update `vpn://`.
- Change watched countries.
- Change poll interval.
- Configure Telegram.
- Download/export redacted diagnostics JSON.

Diagnostics must redact:

- full `vpn://` key
- Amnezia `auth_data.api_key`
- Telegram bot token
- any native WireGuard/AmneziaWG config material

## Storage

Config file:

```text
/opt/etc/awg-watcher/config.json
```

State file:

```text
/opt/var/lib/awg-watcher/state.json
```

Security requirements:

- Config file mode: `0600`.
- State directory and config directory owned by root in Entware.
- Web UI requires an admin password or generated setup token.
- Store web password as a hash, not plaintext.
- Do not log secrets.
- Do not show full secrets in the UI.

Example v1 config shape:

```json
{
  "listen_addr": "127.0.0.1:8097",
  "vpn_key": "vpn://...",
  "countries": ["EE", "NL"],
  "poll_interval_hours": 6,
  "telegram": {
    "bot_token": "...",
    "chat_id": "..."
  },
  "amnezia": {
    "gateway_endpoint": "http://gw.amnezia.org:80/"
  }
}
```

## Amnezia Details Discovered

### `vpn://` Encoding

The initial implementation can use a local decoder script as a reference.

The observed format:

- Strip optional `vpn://` prefix.
- Base64 URL-safe decode, adding padding if needed.
- Native Amnezia format starts with four bytes containing original JSON length, big-endian.
- The remaining bytes are zlib-compressed JSON.
- Some exported configs may be plain base64 JSON without zlib.

The Python script currently accepts both compressed and plain JSON forms.

### Premium V2 Key Shape

The Amnezia client source confirms Premium v2 keys can contain:

```json
{
  "api_config": {
    "service_type": "amnezia-premium",
    "service_protocol": "...",
    "user_country_code": "..."
  },
  "auth_data": {
    "api_key": "..."
  }
}
```

Relevant upstream source files inspected:

- `client/core/models/api/apiConfig.cpp`
- `client/core/models/api/authData.cpp`
- `client/core/utils/constants/apiKeys.h`
- `client/core/utils/api/apiUtils.cpp`
- `client/core/controllers/api/subscriptionController.cpp`
- `client/core/controllers/gatewayController.cpp`

Upstream repository:

```text
https://github.com/amnezia-vpn/amnezia-client
```

### Gateway Endpoint

The Amnezia client defaults to:

```text
http://gw.amnezia.org:80/
```

The source stores this in `SecureAppSettingsRepository`.

### Gateway API Endpoints Found In Source

The official client uses these gateway endpoints:

- `v1/account_info`
- `v1/config`
- `v1/native_config`
- `v1/revoke_config`
- `v1/revoke_native_config`
- `v1/services`
- `v1/news`
- `v1/renewal_link`

V1 of this project must call only:

```text
v1/account_info
```

It must not call:

- `v1/config`
- `v1/native_config`
- revoke endpoints
- AWG-Manager endpoints

This keeps v1 read-only with respect to Amnezia configs and router tunnels.

### Gateway Request Encryption

Important: the gateway is not plain JSON.

The official client:

1. Generates random AES key, IV, and salt.
2. Builds a key payload containing:

```json
{
  "aes_key": "...base64...",
  "aes_iv": "...base64...",
  "aes_salt": "...base64..."
}
```

3. Encrypts the key payload using the Amnezia gateway RSA public key with PKCS#1 padding.
4. Encrypts the API payload with AES using the generated key/IV/salt.
5. Posts JSON:

```json
{
  "key_payload": "...base64...",
  "api_payload": "...base64..."
}
```

6. Decrypts the response body using the same AES key/IV/salt.

The Amnezia source references `QSimpleCrypto` for exact AES behavior:

```text
client/3rd/QSimpleCrypto
```

That is a git submodule:

```text
https://github.com/amnezia-vpn/QSimpleCrypto.git
```

The submodule was not cloned during planning, so the exact AES mode/padding still needs to be confirmed before implementing real gateway calls.

### Gateway Public Key Detail

The Amnezia source references `PROD_AGW_PUBLIC_KEY`, injected at build time, so the source names the mechanism but does not directly expose the production key.

The installed macOS app at:

```text
/Applications/AmneziaVPN.app
```

contains two PEM public keys in the binary. `strings` showed:

- `http://gw.amnezia.org:80/`
- `v1/account_info`
- `v1/native_config`
- `v1/config`
- `key_payload`
- `api_payload`
- two `-----BEGIN PUBLIC KEY-----` blocks

The first public key near those gateway strings is likely the production gateway public key, but this must be verified with a live read-only `v1/account_info` request before relying on it.

## Account Info Fields Used For Detection

The Amnezia client uses `v1/account_info` for account metadata.

Response fields discovered in UI/model source:

- `available_countries`
- `issued_configs`
- `active_device_count`
- `max_device_count`
- `subscription_end_date`
- `subscription_description`
- `supported_protocols`
- `support_info`

For country config monitoring, inspect `issued_configs` entries where:

```json
{
  "source_type": "country_config"
}
```

Relevant fields:

- `server_country_code`
- `server_country_name`
- `installation_uuid`
- `worker_last_updated`
- `last_downloaded`
- `source_type`
- `os_version`

The change detector should compare, per watched country:

- `server_country_code`
- `worker_last_updated`
- `last_downloaded`

If a country disappears from `issued_configs`, report that as a warning/change.

## Detection Flow

1. Load config.
2. Decode `vpn://`.
3. Extract Premium v2 API fields.
4. Build encrypted `v1/account_info` request.
5. Parse decrypted response JSON.
6. Filter `issued_configs` to `source_type = country_config`.
7. Compare watched countries against saved state.
8. On first successful check, save baseline and report "monitoring started".
9. On later checks:
   - if metadata changed, mark country as changed and send Telegram notification.
   - if metadata is same, mark OK.
   - if API errors repeat, show/notify error without destroying last good baseline.

## Telegram Notifications

V1 notification channel: Telegram Bot API.

Required settings:

- bot token
- chat ID

Notifications:

- setup/test notification
- first successful baseline created
- country config metadata changed
- watched country missing from account info
- repeated API failures

Implementation should support a fake Telegram HTTP server for tests.

## OPKG Packaging

Package should install:

- binary: `/opt/bin/awg-watcher`
- init script: `/opt/etc/init.d/S??awg-watcher`
- config directory: `/opt/etc/awg-watcher`
- state directory: `/opt/var/lib/awg-watcher`
- optional cron helper or documented cron line

The init script should support:

```sh
/opt/etc/init.d/S??awg-watcher start
/opt/etc/init.d/S??awg-watcher stop
/opt/etc/init.d/S??awg-watcher restart
```

The binary should also support CLI utilities:

```sh
awg-watcher serve
awg-watcher check
awg-watcher decode
awg-watcher notify-test
awg-watcher status
```

## Safe Testing Plan

### Stage 1: Offline Unit Tests

No network. No router changes.

Test:

- `vpn://` decoding with compressed and plain JSON fixtures.
- Premium v2 field extraction.
- account info parsing.
- country metadata diffing.
- redaction of secrets.
- Telegram request construction against fake HTTP server.
- web handlers using fixture data.

### Stage 2: Local Fixture Web App

Run on a development workstation with fixture `account_info` JSON.

Expected:

- setup page loads.
- fixture countries appear.
- selected countries save to local temp config.
- first check creates baseline.
- modified fixture produces "changed" status.
- no Amnezia network calls occur.

### Stage 3: Real Amnezia Read-Only Test

Use a real `vpn://` key.

Allowed call:

```text
v1/account_info
```

Forbidden calls:

- `v1/native_config`
- `v1/config`
- revoke endpoints
- AWG-Manager endpoints

Expected:

- account info response decrypts successfully.
- available countries and issued configs parse.
- no new native config is issued.
- no router tunnel changes occur.

### Stage 4: Router Smoke Test

Install OPKG on Keenetic.

Keep bind local-only:

```text
127.0.0.1:8097
```

Access via SSH tunnel:

```sh
ssh -L 8097:127.0.0.1:8097 root@router.example
```

Verify:

- Web UI loads.
- setup saves config with mode `0600`.
- "Check now" works.
- state file is written.
- Telegram test notification works.
- no AWG-Manager state changes.

### Stage 5: Controlled Change Simulation

Use fixture mode or edit a copied state file.

Verify:

- UI reports changed country.
- Telegram notification is sent.
- no AWG config is modified.
- last good baseline is preserved if simulated API errors occur.

## Future Phases

### Phase 2: Native Config Export

Add manual export through:

```text
v1/native_config
```

Still do not automatically replace AWG-Manager configs.

### Phase 3: AWG-Manager Replacement

Add AWG-Manager integration:

- configure AWG-Manager base URL and credentials.
- list tunnels.
- backup current tunnel config.
- replace selected tunnel only after explicit confirmation.
- show dry-run preview before replacement.

Implemented in the current app as a manual workflow:

- Settings store AWG-Manager base URL, login, and password.
- The AWG Manager page can test the connection and load tunnels.
- A pasted AmneziaWG `.conf` is previewed locally with key material redacted.
- Replacement first exports the current tunnel config into the watcher backup directory with `0600` mode.
- Replacement is sent to AWG-Manager only after explicit confirmation.

Scheduled monitoring remains read-only and does not call AWG-Manager.

### Phase 4: Fully Automated Replacement

Enable automatic AWG replacement only after repeated successful manual runs and with clear rollback/backup behavior.

## Open Technical Items Before Implementation

- Clone/inspect `QSimpleCrypto` to confirm exact AES mode, padding, and KDF behavior.
- Verify which embedded Amnezia public key is production gateway key.
- Verify encrypted `v1/account_info` against a real `vpn://` key.
- Decide OPKG build workflow for local development vs router build target.
- Decide whether web authentication uses generated setup token, password set during first run, or both.
