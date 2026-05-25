#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: $0 <feed-dir>" >&2
	exit 2
fi

feed_dir=$1
packages_file=$feed_dir/Packages
index_file=$feed_dir/index.html
site_dir=$(cd "$feed_dir/../.." && pwd)
feed_dir_abs=$(cd "$feed_dir" && pwd)
feed_rel=${feed_dir_abs#"$site_dir"/}
site_index_file=$site_dir/index.html

checksum() {
	command=$1
	file=$2
	if command -v "$command" >/dev/null 2>&1; then
		"$command" "$file" | awk '{print $1}'
		return
	fi
	case "$command" in
		md5sum) md5 -q "$file" ;;
		sha256sum) shasum -a 256 "$file" | awk '{print $1}' ;;
		*) return 1 ;;
	esac
}

size_bytes() {
	file=$1
	wc -c < "$file" | tr -d ' '
}

package_control() {
	package=$1
	tar -xOf "$package" ./control.tar.gz | tar -xzOf - ./control
}

: > "$packages_file"

found=0
for package in "$feed_dir"/*.ipk; do
	if [ ! -f "$package" ]; then
		continue
	fi
	found=1
	filename=$(basename "$package")
	control=$(package_control "$package")
	{
		printf '%s\n' "$control"
		printf 'Filename: %s\n' "$filename"
		printf 'Size: %s\n' "$(size_bytes "$package")"
		printf 'MD5Sum: %s\n' "$(checksum md5sum "$package")"
		printf 'SHA256sum: %s\n' "$(checksum sha256sum "$package")"
		printf '\n'
	} >> "$packages_file"
done

if [ "$found" -eq 0 ]; then
	echo "no .ipk files found in $feed_dir" >&2
	exit 1
fi

gzip -9c "$packages_file" > "$packages_file.gz"

cat > "$index_file" <<EOF
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>awg-watcher OPKG feed</title>
</head>
<body>
  <h1>awg-watcher OPKG feed</h1>
  <ul>
    <li><a href="Packages">Packages</a></li>
    <li><a href="Packages.gz">Packages.gz</a></li>
EOF

for package in "$feed_dir"/*.ipk; do
	if [ -f "$package" ]; then
		filename=$(basename "$package")
		printf '    <li><a href="%s">%s</a></li>\n' "$filename" "$filename" >> "$index_file"
	fi
done

cat >> "$index_file" <<EOF
  </ul>
</body>
</html>
EOF

cat > "$site_index_file" <<EOF
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>awg-watcher package feed</title>
</head>
<body>
  <h1>awg-watcher package feed</h1>
  <p><a href="$feed_rel/">Open the OPKG feed</a></p>
</body>
</html>
EOF

echo "$packages_file"
echo "$packages_file.gz"
echo "$index_file"
echo "$site_index_file"
