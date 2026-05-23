# Entware Packaging Notes

The package should install:

- `/opt/bin/amnezia-config-watch`
- `/opt/etc/init.d/S97amnezia-config-watch`
- `/opt/etc/amnezia-config-watch/`
- `/opt/var/lib/amnezia-config-watch/`

Build a Keenetic/Entware-oriented binary from macOS with:

```sh
make build-linux
```

The exact OPKG feed integration depends on your Entware build environment. Until an `.ipk`
recipe is added, copy `bin/amnezia-config-watch-mipsle` to `/opt/bin/amnezia-config-watch`
and copy `packaging/entware/S97amnezia-config-watch` to `/opt/etc/init.d/`.
