#!/bin/sh
set -eu

if [ "$#" -ne 6 ]; then
	echo "usage: $0 <app> <version> <arch> <binary> <init-script> <output-dir>" >&2
	exit 2
fi

app=$1
version=$2
arch=$3
binary=$4
init_script=$5
output_dir=$6

if [ ! -x "$binary" ]; then
	echo "binary not found or not executable: $binary" >&2
	exit 1
fi

if [ ! -f "$init_script" ]; then
	echo "init script not found: $init_script" >&2
	exit 1
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT TERM
export COPYFILE_DISABLE=1
export COPY_EXTENDED_ATTRIBUTES_DISABLE=1

control_dir=$tmpdir/control
data_dir=$tmpdir/data
tar_flags="--format ustar --uid 0 --gid 0 --uname root --gname root"
if tar --help 2>/dev/null | grep -q -- '--no-xattrs'; then
	tar_flags="$tar_flags --no-xattrs"
fi
if tar --help 2>/dev/null | grep -q -- '--no-mac-metadata'; then
	tar_flags="$tar_flags --no-mac-metadata"
fi
mkdir -p "$control_dir" "$data_dir/opt/bin" "$data_dir/opt/etc/init.d"

install -m 0755 "$binary" "$data_dir/opt/bin/$app"
install -m 0755 "$init_script" "$data_dir/opt/etc/init.d/S97$app"

installed_size=$(du -sk "$data_dir" | awk '{print $1}')

cat > "$control_dir/control" <<EOF
Package: $app
Version: $version
Architecture: $arch
Maintainer: awg-watcher maintainers
Section: net
Priority: optional
Installed-Size: $installed_size
Description: Local Keenetic/Entware web app that watches Amnezia Premium country config metadata.
EOF

cat > "$control_dir/postinst" <<EOF
#!/bin/sh
set -e
mkdir -p /opt/etc/$app /opt/var/lib/$app
chmod 700 /opt/etc/$app /opt/var/lib/$app
chmod 755 /opt/bin/$app /opt/etc/init.d/S97$app
list_file=/opt/lib/opkg/info/$app.list
if [ -f "\$list_file" ] && grep -Eq '(^|/)\\._|(^|/)PaxHeader(/|$)' "\$list_file"; then
	tmp_file=\${list_file}.tmp
	grep -Ev '(^|/)\\._|(^|/)PaxHeader(/|$)' "\$list_file" > "\$tmp_file"
	mv "\$tmp_file" "\$list_file"
fi
exit 0
EOF

cat > "$control_dir/prerm" <<EOF
#!/bin/sh
if [ -x /opt/etc/init.d/S97$app ]; then
	/opt/etc/init.d/S97$app stop >/dev/null 2>&1 || true
fi
exit 0
EOF

chmod 0755 "$control_dir/postinst" "$control_dir/prerm"

mkdir -p "$output_dir"
output_dir=$(cd "$output_dir" && pwd)
package_file=$output_dir/${app}_${version}_${arch}.ipk

printf '2.0\n' > "$tmpdir/debian-binary"
(cd "$control_dir" && tar $tar_flags -czf "$tmpdir/control.tar.gz" .)
(cd "$data_dir" && tar $tar_flags -czf "$tmpdir/data.tar.gz" .)
(cd "$tmpdir" && tar $tar_flags -czf "$package_file" ./debian-binary ./control.tar.gz ./data.tar.gz)

echo "$package_file"
