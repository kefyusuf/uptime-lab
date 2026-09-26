#!/usr/bin/env python3
import json
import pathlib
import subprocess
import sys
import tomllib

EXPECTED_CRATES = {
    "checker-core": "crates/checker-core",
    "probe-http": "crates/probe-http",
    "control-plane-client": "crates/control-plane-client",
    "checker": "crates/checker",
}
FORBIDDEN_DB_DEPS = {"postgres", "sqlx", "diesel", "tokio-postgres"}
FORBIDDEN_CONFIG_TOKENS = {
    "allow_private_network",
    "allowprivatenetwork",
    "allow-private-network",
    "private_network_bypass",
    "disable_ssrf",
    "skip_ssrf",
}

class InvariantError(Exception):
    pass

def fail(message: str) -> None:
    raise InvariantError(message)

def main() -> int:
    root = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    checker = root / "apps" / "checker"
    manifest = checker / "Cargo.toml"
    lockfile = checker / "Cargo.lock"
    toolchain = checker / "rust-toolchain.toml"

    for path in (manifest, lockfile, toolchain):
        if not path.is_file():
            fail(f"required Checker file is missing: {path.relative_to(root)}")

    config = tomllib.loads(toolchain.read_text())
    channel = config.get("toolchain", {}).get("channel")
    if channel != "1.98.1":
        fail(f"Rust toolchain must be exactly 1.98.1, got {channel!r}")

    components = set(config.get("toolchain", {}).get("components", []))
    if components != {"rustfmt", "clippy"}:
        fail(f"Rust toolchain components must be exactly rustfmt and clippy, got {sorted(components)}")

    for forbidden in ("shared", "common"):
        if (checker / "crates" / forbidden).exists():
            fail(f"root Checker business crate crates/{forbidden} is forbidden")

    actual_crate_dirs = {
        path.relative_to(checker).as_posix()
        for path in (checker / "crates").iterdir()
        if path.is_dir()
    } if (checker / "crates").is_dir() else set()
    expected_dirs = set(EXPECTED_CRATES.values())
    if actual_crate_dirs != expected_dirs:
        fail(f"Checker crate directories must be exactly {sorted(expected_dirs)}, got {sorted(actual_crate_dirs)}")

    result = subprocess.run(
        ["cargo", "metadata", "--format-version", "1", "--no-deps", "--locked"],
        cwd=checker,
        text=True,
        capture_output=True,
    )
    if result.returncode != 0:
        fail(f"cargo metadata failed: {(result.stderr or result.stdout).strip()}")

    metadata = json.loads(result.stdout)
    packages = {package["name"]: package for package in metadata["packages"]}
    if set(packages) != set(EXPECTED_CRATES):
        fail(f"workspace package set must be exactly {sorted(EXPECTED_CRATES)}, got {sorted(packages)}")

    checker_root = checker.resolve()
    for name, relative in EXPECTED_CRATES.items():
        package = packages[name]
        manifest_path = pathlib.Path(package["manifest_path"]).resolve()
        expected_manifest = (checker / relative / "Cargo.toml").resolve()
        if manifest_path != expected_manifest:
            fail(f"{name} manifest path must be {expected_manifest.relative_to(root)}")
        try:
            manifest_path.relative_to(checker_root)
        except ValueError:
            fail(f"{name} manifest must remain inside apps/checker")

        dependencies = {dep["name"] for dep in package.get("dependencies", [])}
        forbidden_db = dependencies & FORBIDDEN_DB_DEPS
        if forbidden_db:
            fail(f"{name} has forbidden database dependencies: {sorted(forbidden_db)}")

        if name == "checker-core":
            forbidden = dependencies & {"probe-http", "control-plane-client", "checker"}
            if forbidden:
                fail(f"checker-core must not depend outward on {sorted(forbidden)}")
        elif name == "probe-http":
            forbidden = dependencies & {"control-plane-client", "checker"}
            if forbidden:
                fail(f"probe-http has forbidden outward dependencies: {sorted(forbidden)}")
        elif name == "control-plane-client":
            forbidden = dependencies & {"probe-http", "checker"}
            if forbidden:
                fail(f"control-plane-client has forbidden outward dependencies: {sorted(forbidden)}")

    for path in checker.rglob("*"):
        if not path.is_file() or path.name == "Cargo.lock":
            continue
        if path.suffix not in {".rs", ".toml"}:
            continue
        lowered = path.read_text(errors="replace").lower()
        for token in FORBIDDEN_CONFIG_TOKENS:
            if token in lowered:
                fail(f"known private-network bypass token {token!r} is forbidden in {path.relative_to(root)}")

    print("Rust architecture check: PASS")
    return 0

if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except InvariantError as error:
        print(f"Rust architecture invariant failed: {error}", file=sys.stderr)
        raise SystemExit(1)
