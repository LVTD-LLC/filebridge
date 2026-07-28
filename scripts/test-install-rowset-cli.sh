#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT INT TERM

fixture_dir="$test_root/fixtures"
mock_bin="$test_root/mock-bin"
install_dir="$test_root/install"
package_dir="$test_root/package"
mkdir -p "$fixture_dir" "$mock_bin" "$install_dir" "$package_dir"

printf '#!/usr/bin/env sh\nprintf "rowset fixture\\n"\n' > "$package_dir/rowset"
chmod 0755 "$package_dir/rowset"
case "$(uname -s)" in
	Linux) test_os="linux" ;;
	Darwin) test_os="darwin" ;;
	*) printf 'unsupported test OS\n' >&2; exit 1 ;;
esac
case "$(uname -m)" in
	x86_64 | amd64) test_arch="amd64" ;;
	arm64 | aarch64) test_arch="arm64" ;;
	*) printf 'unsupported test architecture\n' >&2; exit 1 ;;
esac
archive="$fixture_dir/rowset_${test_os}_${test_arch}.tar.gz"
tar -C "$package_dir" -czf "$archive" rowset

checksum() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

printf '%s  %s\n' "$(checksum "$archive")" "$(basename "$archive")" \
	> "$fixture_dir/checksums.txt"

cat > "$mock_bin/curl" <<'MOCK'
#!/usr/bin/env sh
set -eu

url=""
output=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-o)
			output="$2"
			shift 2
			;;
		-*)
			shift
			;;
		*)
			url="$1"
			shift
			;;
	esac
done
cp "$ROWSET_TEST_FIXTURES/$(basename "$url")" "$output"
MOCK
chmod 0755 "$mock_bin/curl"

run_installer() {
	PATH="$mock_bin:$PATH" \
	ROWSET_TEST_FIXTURES="$fixture_dir" \
	ROWSET_CLI_REPO="example/rowset" \
	ROWSET_CLI_VERSION="test-version" \
	ROWSET_INSTALL_DIR="$install_dir" \
		sh "$repo_root/scripts/install-rowset-cli.sh"
}

run_installer >/dev/null
installed_output="$("$install_dir/rowset")"
if [ "$installed_output" != "rowset fixture" ]; then
	printf 'installed binary mismatch: %s\n' "$installed_output" >&2
	exit 1
fi

printf '#!/usr/bin/env sh\nprintf "previous install\\n"\n' > "$install_dir/rowset"
chmod 0755 "$install_dir/rowset"
printf '%064d  %s\n' 0 "$(basename "$archive")" > "$fixture_dir/checksums.txt"

if run_installer >/dev/null 2>&1; then
	printf 'installer accepted an archive with a mismatched checksum\n' >&2
	exit 1
fi
installed_output="$("$install_dir/rowset")"
if [ "$installed_output" != "previous install" ]; then
	printf 'failed verification replaced the existing install\n' >&2
	exit 1
fi

printf 'unexpected\n' > "$package_dir/extra"
tar -C "$package_dir" -czf "$archive" rowset extra
printf '%s  %s\n' "$(checksum "$archive")" "$(basename "$archive")" \
	> "$fixture_dir/checksums.txt"
if run_installer >/dev/null 2>&1; then
	printf 'installer accepted an archive with extra entries\n' >&2
	exit 1
fi
installed_output="$("$install_dir/rowset")"
if [ "$installed_output" != "previous install" ]; then
	printf 'malformed archive replaced the existing install\n' >&2
	exit 1
fi

mv "$package_dir/rowset" "$package_dir/rowset-real"
ln -s rowset-real "$package_dir/rowset"
tar -C "$package_dir" -czf "$archive" rowset
printf '%s  %s\n' "$(checksum "$archive")" "$(basename "$archive")" \
	> "$fixture_dir/checksums.txt"
if run_installer >/dev/null 2>&1; then
	printf 'installer accepted a symlink as the rowset binary\n' >&2
	exit 1
fi
installed_output="$("$install_dir/rowset")"
if [ "$installed_output" != "previous install" ]; then
	printf 'unsafe archive replaced the existing install\n' >&2
	exit 1
fi

printf 'installer checksum tests passed\n'
