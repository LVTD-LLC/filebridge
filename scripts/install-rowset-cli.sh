#!/usr/bin/env sh
set -eu

repo="${ROWSET_CLI_REPO:-LVTD-LLC/rowset}"
version="${ROWSET_CLI_VERSION:-latest}"
install_dir="${ROWSET_INSTALL_DIR:-}"
default_latest_url="https://github.com/LVTD-LLC/rowset/releases/latest/download"

detect_os() {
	case "$(uname -s)" in
		Linux) printf "linux" ;;
		Darwin) printf "darwin" ;;
		*) printf "Unsupported OS: %s\n" "$(uname -s)" >&2; exit 1 ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
		x86_64 | amd64) printf "amd64" ;;
		arm64 | aarch64) printf "arm64" ;;
		*) printf "Unsupported architecture: %s\n" "$(uname -m)" >&2; exit 1 ;;
	esac
}

choose_install_dir() {
	if [ -n "$install_dir" ]; then
		printf "%s" "$install_dir"
		return
	fi

	if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
		printf "/usr/local/bin"
		return
	fi

	printf "%s/.local/bin" "$HOME"
}

download() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1" -o "$2"
		return
	fi
	if command -v wget >/dev/null 2>&1; then
		wget -qO "$2" "$1"
		return
	fi
	printf "curl or wget is required to install rowset.\n" >&2
	exit 1
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
		return
	fi
	if command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
		return
	fi
	printf "sha256sum or shasum is required to verify rowset.\n" >&2
	exit 1
}

os="$(detect_os)"
arch="$(detect_arch)"
asset="rowset_${os}_${arch}.tar.gz"
if [ "$version" = "latest" ]; then
	if [ "$repo" = "LVTD-LLC/rowset" ]; then
		base_url="$default_latest_url"
	else
		base_url="https://github.com/${repo}/releases/latest/download"
	fi
else
	base_url="https://github.com/${repo}/releases/download/${version}"
fi
url="${base_url}/${asset}"

tmp_dir="$(mktemp -d)"
install_tmp=""
cleanup() {
	rm -rf "$tmp_dir"
	if [ -n "$install_tmp" ]; then
		rm -f "$install_tmp"
	fi
}
trap cleanup EXIT INT TERM

download "${base_url}/checksums.txt" "$tmp_dir/checksums.txt"
expected_checksum="$(
	awk -v asset="$asset" '
		$2 == asset || $2 == "*" asset {
			print $1
			exit
		}
	' "$tmp_dir/checksums.txt"
)"
case "$expected_checksum" in
	????????????????????????????????????????????????????????????????) ;;
	*)
		printf "No valid SHA-256 checksum was published for %s.\n" "$asset" >&2
		exit 1
		;;
esac
download "$url" "$tmp_dir/$asset"
actual_checksum="$(sha256 "$tmp_dir/$asset")"
if [ "$actual_checksum" != "$expected_checksum" ]; then
	printf "Checksum verification failed for %s.\n" "$asset" >&2
	exit 1
fi

archive_entries="$(tar -tzf "$tmp_dir/$asset")"
if [ "$archive_entries" != "rowset" ]; then
	printf "Unexpected files in %s.\n" "$asset" >&2
	exit 1
fi
tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"
if [ ! -f "$tmp_dir/rowset" ] || [ -L "$tmp_dir/rowset" ]; then
	printf "Archive did not contain a regular rowset binary.\n" >&2
	exit 1
fi

install_dir="$(choose_install_dir)"
mkdir -p "$install_dir"
install_tmp="$(mktemp "$install_dir/.rowset.tmp.XXXXXX")"
install -m 0755 "$tmp_dir/rowset" "$install_tmp"
mv -f "$install_tmp" "$install_dir/rowset"
install_tmp=""

printf "rowset installed to %s/rowset\n" "$install_dir"
case ":$PATH:" in
	*":$install_dir:"*) ;;
	*) printf "Add %s to PATH before running rowset.\n" "$install_dir" ;;
esac
