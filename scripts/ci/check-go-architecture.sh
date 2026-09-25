#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
API_ROOT="$ROOT/apps/api"
GO_MOD="$API_ROOT/go.mod"
EXPECTED_MODULE="github.com/kefyusuf/uptime-lab/apps/api"

fail() {
  printf 'Go architecture invariant failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$GO_MOD" ]] || fail "apps/api/go.mod is missing"

MODULE="$(awk '$1 == "module" { print $2; exit }' "$GO_MOD")"
[[ "$MODULE" == "$EXPECTED_MODULE" ]] || fail "Go module must be $EXPECTED_MODULE"

for bucket in shared common; do
  [[ ! -d "$API_ROOT/internal/$bucket" ]] || fail "root business bucket internal/$bucket is forbidden"
done

MONITORING="$MODULE/internal/modules/monitoring"
DOMAIN="$MONITORING/domain"
APPLICATION="$MONITORING/application"
PORTS="$MONITORING/ports"
ADAPTERS="$MONITORING/adapters"
HTTP_ADAPTER="$ADAPTERS/http"
POSTGRES_ADAPTER="$ADAPTERS/postgres"
PLATFORM="$MODULE/internal/platform"

is_path_or_child() {
  local value="$1"
  local prefix="$2"
  [[ "$value" == "$prefix" || "$value" == "$prefix/"* ]]
}

is_pgx() {
  local value="$1"
  [[ "$value" == "github.com/jackc/pgx" || "$value" == "github.com/jackc/pgx/"* ]]
}

is_goose() {
  local value="$1"
  [[ "$value" == "github.com/pressly/goose" || "$value" == "github.com/pressly/goose/"* ]]
}

reject_import() {
  local package="$1"
  local imported="$2"
  local reason="$3"
  fail "$package must not import $imported ($reason)"
}

PACKAGE_ROWS=""
if ! PACKAGE_ROWS="$(cd "$API_ROOT" && go list -f '{{.ImportPath}}|{{join .Imports ","}}' ./... 2>&1)"; then
  fail "go list failed: $PACKAGE_ROWS"
fi

while IFS='|' read -r package imports; do
  [[ -n "$package" ]] || continue

  IFS=',' read -r -a import_list <<<"$imports"
  for imported in "${import_list[@]}"; do
    [[ -n "$imported" ]] || continue

    if is_path_or_child "$package" "$DOMAIN"; then
      case "$imported" in
        net/http)
          reject_import "$package" "$imported" "domain must not depend on HTTP transport"
          ;;
      esac

      if is_path_or_child "$imported" "$APPLICATION"; then
        reject_import "$package" "$imported" "domain must not depend on application"
      fi
      if is_path_or_child "$imported" "$PORTS"; then
        reject_import "$package" "$imported" "domain must not depend on ports"
      fi
      if is_path_or_child "$imported" "$ADAPTERS"; then
        reject_import "$package" "$imported" "domain must not depend on adapters"
      fi
      if is_path_or_child "$imported" "$PLATFORM"; then
        reject_import "$package" "$imported" "domain must not depend on platform"
      fi
      if is_pgx "$imported"; then
        reject_import "$package" "$imported" "domain must not depend on pgx"
      fi
      if is_goose "$imported"; then
        reject_import "$package" "$imported" "domain must not depend on goose"
      fi
    fi

    if is_path_or_child "$package" "$APPLICATION"; then
      case "$imported" in
        net/http)
          reject_import "$package" "$imported" "application must not depend on HTTP transport"
          ;;
      esac

      if is_path_or_child "$imported" "$ADAPTERS"; then
        reject_import "$package" "$imported" "application must not depend on adapters"
      fi
      if is_path_or_child "$imported" "$PLATFORM"; then
        reject_import "$package" "$imported" "application must not depend on platform"
      fi
      if is_pgx "$imported"; then
        reject_import "$package" "$imported" "application must not depend on pgx"
      fi
      if is_goose "$imported"; then
        reject_import "$package" "$imported" "application must not depend on goose"
      fi
    fi

    if is_path_or_child "$package" "$PORTS"; then
      if is_path_or_child "$imported" "$APPLICATION"; then
        reject_import "$package" "$imported" "ports must not depend on application"
      fi
      if is_path_or_child "$imported" "$ADAPTERS"; then
        reject_import "$package" "$imported" "ports must not depend on adapters"
      fi
      if is_path_or_child "$imported" "$PLATFORM"; then
        reject_import "$package" "$imported" "ports must not depend on platform"
      fi
      if is_pgx "$imported"; then
        reject_import "$package" "$imported" "ports must not depend on pgx"
      fi
      if is_goose "$imported"; then
        reject_import "$package" "$imported" "ports must not depend on goose"
      fi
    fi

    if is_path_or_child "$package" "$PLATFORM"; then
      if is_path_or_child "$imported" "$MONITORING"; then
        reject_import "$package" "$imported" "platform must remain business-module agnostic"
      fi
    fi

    if is_path_or_child "$package" "$HTTP_ADAPTER"; then
      if is_path_or_child "$imported" "$POSTGRES_ADAPTER"; then
        reject_import "$package" "$imported" "Monitoring HTTP adapter must not depend on postgres adapter"
      fi
      if is_path_or_child "$imported" "$PLATFORM"; then
        reject_import "$package" "$imported" "Monitoring HTTP adapter must not depend on platform"
      fi
      if is_pgx "$imported"; then
        reject_import "$package" "$imported" "Monitoring HTTP adapter must not depend on pgx"
      fi
      if is_goose "$imported"; then
        reject_import "$package" "$imported" "Monitoring HTTP adapter must not depend on goose"
      fi
    fi
  done
done <<<"$PACKAGE_ROWS"
