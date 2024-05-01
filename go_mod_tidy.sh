#!/bin/bash

set -euo pipefail

if [[ -z "${BUILD_WORKSPACE_DIRECTORY:-}" ]]; then
	echo >&2 "This script must be run under bazel - please run \`bazel run //:go_mod_tidy\`"
	exit 1
fi

# --- begin runfiles.bash initialization v3 ---
# Copy-pasted from the Bazel Bash runfiles library v3.
set -uo pipefail
set +e
f=bazel_tools/tools/bash/runfiles/runfiles.bash
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null ||
	source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null ||
	source "$0.runfiles/$f" 2>/dev/null ||
	source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null ||
	source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null ||
	{
		echo >&2 "ERROR: cannot find $f"
		exit 1
	}
f=
set -e
# --- end runfiles.bash initialization v3 ---

GO="$(rlocation "${GO}")"

cd "${BUILD_WORKSPACE_DIRECTORY}"

echo "package bazel_flags" >bazel_protos/bazel_flags/dummy_for_go_mod_tidy.go
cleanup() {
	rm bazel_protos/bazel_flags/dummy_for_go_mod_tidy.go
}
trap cleanup EXIT

"${GO}" mod tidy
