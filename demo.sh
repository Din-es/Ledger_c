#!/usr/bin/env bash
# End-to-end walkthrough of the decision ledger against a throwaway repo.
# Usage: ./demo.sh [path-to-ledger-binary]
set -euo pipefail

LEDGER="${1:-$(pwd)/ledger.exe}"
[ -x "$LEDGER" ] || LEDGER="${1:-$(pwd)/ledger}"
DEMO="$(mktemp -d)/ledger-demo"

step() { printf '\n\033[1m── %s\033[0m\n' "$1"; }
# Runs a command, echoing it first, and stashes its exit code in LAST so the
# script can narrate whether the CI gate blocked or passed.
LAST=0
run()  { printf '\033[2m$ %s\033[0m\n' "$*"; "$@" && LAST=0 || LAST=$?; }
verdict() { [ "$LAST" -eq 0 ] && printf '\033[2m→ exit 0 — merge allowed\033[0m\n' \
                             || printf '\033[2m→ exit %s — PR blocked\033[0m\n' "$LAST"; }

mkdir -p "$DEMO/docs/decisions" && cd "$DEMO"
git init -q
git config user.email demo@example.com
git config user.name demo
git config core.autocrlf false

cat > retry.go <<'EOF'
package auth

import "time"

func retry(fn func() error) error {
	var err error
	for i := 0; i < 5; i++ {
		if err = fn(); err == nil {
			return nil
		}
		// jittered backoff: base*2^i +/- 25% to avoid thundering herd
		d := backoff(i)
		time.Sleep(d)
	}
	return err
}
EOF

cat > docs/decisions/jitter-backoff.md <<'EOF'
# Jittered backoff

Retries jitter by +/- 25% so a fleet restarting together doesn't
synchronize into a thundering herd against the auth service.
EOF

git add -A && git commit -qm "auth retry with jittered backoff"

step "1. Bind the decision to the code it governs"
run "$LEDGER" bind retry.go:11-13 --note jitter-backoff \
    --title "Jittered backoff avoids thundering herd"
git add .ledger && git commit -qm "record the jitter decision"
BASE=$(git rev-parse HEAD)

step "2. Someone refactors — 12 lines of docs land above the code"
{ printf '// Package auth handles credential refresh and retry.\n'
  printf '//\n'
  for i in $(seq 1 10); do printf '// (boilerplate line %s)\n' "$i"; done
  cat retry.go; } > tmp && mv tmp retry.go
git commit -qam "add package documentation"
run "$LEDGER" resolve jitter-backoff

step "3. Code moves to a new package — the file is renamed"
mkdir -p internal/auth && git mv retry.go internal/auth/retry.go
git commit -qm "move auth into internal/"
run "$LEDGER" resolve jitter-backoff

step "4. A PR changes the governed code but not the rationale"
sed -i 's/25%/50%/' internal/auth/retry.go
git commit -qam "widen jitter to 50%"
run "$LEDGER" verify --since "$BASE"; verdict

step "5. The author revisits the decision note — gate clears"
cat >> docs/decisions/jitter-backoff.md <<'EOF'

Updated: widened to +/- 50% after load testing showed 25% still
produced visible retry spikes at 10k concurrent clients.
EOF
git commit -qam "revisit jitter rationale"
run "$LEDGER" verify --since "$BASE"; verdict

step "6. Reading the code later: why does this exist?"
run "$LEDGER" why internal/auth/retry.go:23

step "7. Someone deletes the mechanism entirely"
sed -i '23,25d' internal/auth/retry.go
git commit -qam "drop jitter, use fixed sleep"
run "$LEDGER" verify; verdict

printf '\n\033[2mdemo repo: %s\033[0m\n' "$DEMO"
