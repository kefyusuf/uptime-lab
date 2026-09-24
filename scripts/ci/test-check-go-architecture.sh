#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$SCRIPT_DIR/check-go-architecture.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

PASS=0
FAIL=0

pass() { printf 'PASS: %s\n' "$1"; PASS=$((PASS + 1)); }
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=$((FAIL + 1)); }

expect_success() {
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then pass "$name"; else fail "$name"; fi
}

expect_failure() {
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then fail "$name"; else pass "$name"; fi
}

write_file() {
  local path="$1"
  local content="$2"
  mkdir -p "$(dirname "$path")"
  printf '%s\n' "$content" > "$path"
}

make_fixture() {
  rm -rf "$TMP/repo"
  mkdir -p "$TMP/repo/apps/api/internal/modules/monitoring/domain"
  mkdir -p "$TMP/repo/apps/api/internal/modules/monitoring/ports"
  mkdir -p "$TMP/repo/apps/api/internal/modules/monitoring/application"

  cat > "$TMP/repo/apps/api/go.mod" <<'MOD'
module github.com/kefyusuf/uptime-lab/apps/api

go 1.27.1
MOD

  cat > "$TMP/repo/apps/api/internal/modules/monitoring/domain/domain.go" <<'GO'
package domain

type Monitor struct{}
GO

  cat > "$TMP/repo/apps/api/internal/modules/monitoring/ports/ports.go" <<'GO'
package ports

import "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"

type MonitorRepository interface {
	Create(domain.Monitor) error
}
GO

  cat > "$TMP/repo/apps/api/internal/modules/monitoring/application/application.go" <<'GO'
package application

import (
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

var _ domain.Monitor
var _ ports.MonitorRepository
GO
}

make_simple_package() {
  local path="$1"
  local package_name="$2"
  mkdir -p "$TMP/repo/apps/api/$path"
  printf 'package %s\n' "$package_name" > "$TMP/repo/apps/api/$path/package.go"
}

set_application_minimal() {
  cat > "$TMP/repo/apps/api/internal/modules/monitoring/application/application.go" <<'GO'
package application
GO
}

make_fixture
expect_success "canonical monitoring layout passes" "$CHECKER" "$TMP/repo"

make_fixture
set_application_minimal
cat > "$TMP/repo/apps/api/internal/modules/monitoring/domain/bad.go" <<'GO'
package domain

import _ "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
GO
expect_failure "domain importing application fails" "$CHECKER" "$TMP/repo"

make_fixture
make_simple_package "internal/platform/example" "example"
cat > "$TMP/repo/apps/api/internal/modules/monitoring/domain/bad.go" <<'GO'
package domain

import _ "github.com/kefyusuf/uptime-lab/apps/api/internal/platform/example"
GO
expect_failure "domain importing platform fails" "$CHECKER" "$TMP/repo"

make_fixture
make_simple_package "internal/modules/monitoring/adapters/postgres" "postgres"
cat > "$TMP/repo/apps/api/internal/modules/monitoring/domain/bad.go" <<'GO'
package domain

import _ "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/adapters/postgres"
GO
expect_failure "domain importing postgres adapter fails" "$CHECKER" "$TMP/repo"

make_fixture
cat > "$TMP/repo/apps/api/internal/modules/monitoring/domain/bad.go" <<'GO'
package domain

import _ "net/http"
GO
expect_failure "domain importing net/http fails" "$CHECKER" "$TMP/repo"

make_fixture
make_simple_package "internal/modules/monitoring/adapters/postgres" "postgres"
cat > "$TMP/repo/apps/api/internal/modules/monitoring/application/bad.go" <<'GO'
package application

import _ "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/adapters/postgres"
GO
expect_failure "application importing postgres adapter fails" "$CHECKER" "$TMP/repo"

make_fixture
make_simple_package "internal/platform/example" "example"
cat > "$TMP/repo/apps/api/internal/modules/monitoring/application/bad.go" <<'GO'
package application

import _ "github.com/kefyusuf/uptime-lab/apps/api/internal/platform/example"
GO
expect_failure "application importing platform fails" "$CHECKER" "$TMP/repo"

make_fixture
make_simple_package "internal/modules/monitoring/adapters/postgres" "postgres"
cat > "$TMP/repo/apps/api/internal/modules/monitoring/ports/bad.go" <<'GO'
package ports

import _ "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/adapters/postgres"
GO
expect_failure "ports importing concrete adapter fails" "$CHECKER" "$TMP/repo"

make_fixture
make_simple_package "internal/shared" "shared"
expect_failure "root shared business package fails" "$CHECKER" "$TMP/repo"

make_fixture
make_simple_package "internal/common" "common"
expect_failure "root common business package fails" "$CHECKER" "$TMP/repo"

make_fixture
mkdir -p "$TMP/repo/fakes/pgx"
cat >> "$TMP/repo/apps/api/go.mod" <<'MOD'

require github.com/jackc/pgx/v5 v5.0.0
replace github.com/jackc/pgx/v5 => ../../fakes/pgx
MOD
cat > "$TMP/repo/fakes/pgx/go.mod" <<'MOD'
module github.com/jackc/pgx/v5

go 1.27.1
MOD
cat > "$TMP/repo/fakes/pgx/pgx.go" <<'GO'
package pgx
GO
cat > "$TMP/repo/apps/api/internal/modules/monitoring/domain/bad.go" <<'GO'
package domain

import _ "github.com/jackc/pgx/v5"
GO
expect_failure "domain importing pgx fails without network" "$CHECKER" "$TMP/repo"


make_fixture
make_simple_package "internal/platform/example" "example"
cat > "$TMP/repo/apps/api/internal/platform/example/bad.go" <<'GO'
package example

import _ "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
GO
expect_failure "platform importing Monitoring fails" "$CHECKER" "$TMP/repo"

make_fixture
cat > "$TMP/repo/apps/api/internal/modules/monitoring/application/bad.go" <<'GO'
package application

import _ "net/http"
GO
expect_failure "application importing net/http fails" "$CHECKER" "$TMP/repo"

make_fixture
make_simple_package "internal/modules/monitoring/adapters/postgres" "postgres"
make_simple_package "internal/modules/monitoring/adapters/http" "monitoringhttp"
cat > "$TMP/repo/apps/api/internal/modules/monitoring/adapters/http/bad.go" <<'GO'
package monitoringhttp

import _ "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/adapters/postgres"
GO
expect_failure "Monitoring HTTP adapter importing postgres adapter fails" "$CHECKER" "$TMP/repo"

make_fixture
make_simple_package "internal/platform/example" "example"
make_simple_package "internal/modules/monitoring/adapters/http" "monitoringhttp"
cat > "$TMP/repo/apps/api/internal/modules/monitoring/adapters/http/bad.go" <<'GO'
package monitoringhttp

import _ "github.com/kefyusuf/uptime-lab/apps/api/internal/platform/example"
GO
expect_failure "Monitoring HTTP adapter importing platform fails" "$CHECKER" "$TMP/repo"

make_fixture
mkdir -p "$TMP/repo/fakes/pgx"
cat >> "$TMP/repo/apps/api/go.mod" <<'MOD'

require github.com/jackc/pgx/v5 v5.0.0
replace github.com/jackc/pgx/v5 => ../../fakes/pgx
MOD
cat > "$TMP/repo/fakes/pgx/go.mod" <<'MOD'
module github.com/jackc/pgx/v5

go 1.27.1
MOD
cat > "$TMP/repo/fakes/pgx/pgx.go" <<'GO'
package pgx
GO
make_simple_package "internal/modules/monitoring/adapters/http" "monitoringhttp"
cat > "$TMP/repo/apps/api/internal/modules/monitoring/adapters/http/bad.go" <<'GO'
package monitoringhttp

import _ "github.com/jackc/pgx/v5"
GO
expect_failure "Monitoring HTTP adapter importing pgx fails without network" "$CHECKER" "$TMP/repo"

make_fixture
mkdir -p "$TMP/repo/fakes/goose"
cat >> "$TMP/repo/apps/api/go.mod" <<'MOD'

require github.com/pressly/goose/v3 v3.0.0
replace github.com/pressly/goose/v3 => ../../fakes/goose
MOD
cat > "$TMP/repo/fakes/goose/go.mod" <<'MOD'
module github.com/pressly/goose/v3

go 1.27.1
MOD
cat > "$TMP/repo/fakes/goose/goose.go" <<'GO'
package goose
GO
make_simple_package "internal/modules/monitoring/adapters/http" "monitoringhttp"
cat > "$TMP/repo/apps/api/internal/modules/monitoring/adapters/http/bad.go" <<'GO'
package monitoringhttp

import _ "github.com/pressly/goose/v3"
GO
expect_failure "Monitoring HTTP adapter importing goose fails without network" "$CHECKER" "$TMP/repo"

make_fixture
make_simple_package "internal/modules/monitoring/adapters/http" "monitoringhttp"
cat > "$TMP/repo/apps/api/internal/modules/monitoring/adapters/http/inward.go" <<'GO'
package monitoringhttp

import (
	_ "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	_ "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)
GO
expect_success "Monitoring HTTP adapter may import application and domain" "$CHECKER" "$TMP/repo"

make_fixture
make_simple_package "internal/platform/httpserver" "httpserver"
make_simple_package "internal/modules/monitoring/adapters/http" "monitoringhttp"
mkdir -p "$TMP/repo/apps/api/cmd/api"
cat > "$TMP/repo/apps/api/cmd/api/main.go" <<'GO'
package main

import (
	_ "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/adapters/http"
	_ "github.com/kefyusuf/uptime-lab/apps/api/internal/platform/httpserver"
)

func main() {}
GO
expect_success "cmd/api may import platform and Monitoring adapters" "$CHECKER" "$TMP/repo"

printf '\nGo architecture tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$FAIL" -eq 0
