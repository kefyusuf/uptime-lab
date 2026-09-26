#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$SCRIPT_DIR/check-rust-architecture.py"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

PASS=0
FAIL=0

pass() { printf 'PASS: %s\n' "$1"; PASS=$((PASS + 1)); }
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=$((FAIL + 1)); }
expect_success() { local n="$1"; shift; if "$@" >/dev/null 2>&1; then pass "$n"; else fail "$n"; fi; }
expect_failure() { local n="$1"; shift; if "$@" >/dev/null 2>&1; then fail "$n"; else pass "$n"; fi; }

write_crate() {
  local name="$1"
  local deps="${2:-}"
  local path="$TMP/repo/apps/checker/crates/$name"
  mkdir -p "$path/src"
  cat > "$path/Cargo.toml" <<EOF
[package]
name = "$name"
version = "0.1.0"
edition = "2024"
rust-version = "1.98.1"

[dependencies]
$deps
EOF
  if [[ "$name" == "checker" ]]; then
    printf 'fn main() {}\n' > "$path/src/main.rs"
  else
    printf 'pub fn marker() {}\n' > "$path/src/lib.rs"
  fi
}

make_fixture() {
  rm -rf "$TMP/repo"
  mkdir -p "$TMP/repo/apps/checker/crates"
  cat > "$TMP/repo/apps/checker/Cargo.toml" <<'EOF'
[workspace]
resolver = "3"
members = [
  "crates/checker-core",
  "crates/probe-http",
  "crates/control-plane-client",
  "crates/checker",
]
EOF
  cat > "$TMP/repo/apps/checker/rust-toolchain.toml" <<'EOF'
[toolchain]
channel = "1.98.1"
profile = "minimal"
components = ["rustfmt", "clippy"]
EOF
  write_crate "checker-core"
  write_crate "probe-http" 'checker-core = { path = "../checker-core" }'
  write_crate "control-plane-client" 'checker-core = { path = "../checker-core" }'
  write_crate "checker" $'checker-core = { path = "../checker-core" }\nprobe-http = { path = "../probe-http" }\ncontrol-plane-client = { path = "../control-plane-client" }'
  (cd "$TMP/repo/apps/checker" && cargo generate-lockfile --offline >/dev/null)
}

make_fixture
expect_success "canonical four-crate Checker workspace passes" python3 "$CHECKER" "$TMP/repo"

make_fixture
mkdir -p "$TMP/repo/apps/checker/crates/shared/src"
cat > "$TMP/repo/apps/checker/crates/shared/Cargo.toml" <<'EOF'
[package]
name = "shared"
version = "0.1.0"
edition = "2024"
EOF
printf 'pub fn marker() {}\n' > "$TMP/repo/apps/checker/crates/shared/src/lib.rs"
expect_failure "root shared Checker crate fails" python3 "$CHECKER" "$TMP/repo"

make_fixture
sed -i '/checker-core = { path = "..\/checker-core" }/d' "$TMP/repo/apps/checker/crates/probe-http/Cargo.toml"
cat >> "$TMP/repo/apps/checker/crates/checker-core/Cargo.toml" <<'EOF'
probe-http = { path = "../probe-http" }
EOF
(cd "$TMP/repo/apps/checker" && cargo generate-lockfile --offline >/dev/null)
expect_failure "checker-core outward dependency fails" python3 "$CHECKER" "$TMP/repo"

make_fixture
cat >> "$TMP/repo/apps/checker/crates/probe-http/Cargo.toml" <<'EOF'
control-plane-client = { path = "../control-plane-client" }
EOF
(cd "$TMP/repo/apps/checker" && cargo generate-lockfile --offline >/dev/null)
expect_failure "probe-http outward dependency fails" python3 "$CHECKER" "$TMP/repo"

make_fixture
cat >> "$TMP/repo/apps/checker/crates/control-plane-client/Cargo.toml" <<'EOF'
probe-http = { path = "../probe-http" }
EOF
(cd "$TMP/repo/apps/checker" && cargo generate-lockfile --offline >/dev/null)
expect_failure "control-plane-client outward dependency fails" python3 "$CHECKER" "$TMP/repo"

make_fixture
mkdir -p "$TMP/repo/apps/checker/fakes/sqlx/src"
cat > "$TMP/repo/apps/checker/fakes/sqlx/Cargo.toml" <<'EOF'
[package]
name = "sqlx"
version = "0.1.0"
edition = "2024"
EOF
printf 'pub fn marker() {}\n' > "$TMP/repo/apps/checker/fakes/sqlx/src/lib.rs"
cat >> "$TMP/repo/apps/checker/crates/checker/Cargo.toml" <<'EOF'
sqlx = { path = "../../fakes/sqlx" }
EOF
(cd "$TMP/repo/apps/checker" && cargo generate-lockfile --offline >/dev/null)
expect_failure "database dependency fails" python3 "$CHECKER" "$TMP/repo"

make_fixture
printf 'pub const ALLOW_PRIVATE_NETWORK: bool = true;\n' >> "$TMP/repo/apps/checker/crates/probe-http/src/lib.rs"
expect_failure "private-network bypass token fails" python3 "$CHECKER" "$TMP/repo"

make_fixture
sed -i 's/channel = "1.98.1"/channel = "stable"/' "$TMP/repo/apps/checker/rust-toolchain.toml"
expect_failure "floating Rust toolchain fails" python3 "$CHECKER" "$TMP/repo"

printf '\nRust architecture tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$PASS" -eq 8
test "$FAIL" -eq 0
