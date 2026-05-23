# Security

This project stores operational secrets in the local config file. Do not commit a real
`config.json`, `state.json`, Amnezia `vpn://` key, Telegram bot token, gateway public key
extracted from proprietary binaries, native WireGuard/AmneziaWG configs, or diagnostics
from a live account.

The default web listener is `127.0.0.1:8097`. Keep it loopback-only unless you have set
a strong admin password and intentionally want LAN access.

Report security-sensitive issues privately to the repository owner.
