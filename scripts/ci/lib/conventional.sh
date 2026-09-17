#!/usr/bin/env bash

CONVENTIONAL_TYPES='feat|fix|refactor|test|docs|ci|build|chore|perf|revert'
CONVENTIONAL_SCOPES='web|api|checker|contracts|devops|architecture|docs'
CONVENTIONAL_SUBJECT_RE="^(${CONVENTIONAL_TYPES})(\\((${CONVENTIONAL_SCOPES})\\))?(!)?: [a-z0-9][^[:cntrl:]]*$"

is_conventional_subject() {
  local subject="${1:-}"
  [[ "$subject" =~ $CONVENTIONAL_SUBJECT_RE ]]
}

print_conventional_error() {
  local subject="${1:-}"
  printf 'Invalid Conventional Commit subject: %s\n' "$subject" >&2
  printf 'Expected: <type>(<scope>): <lowercase description>\n' >&2
  printf 'Types: feat, fix, refactor, test, docs, ci, build, chore, perf, revert\n' >&2
  printf 'Scopes: web, api, checker, contracts, devops, architecture, docs\n' >&2
}
