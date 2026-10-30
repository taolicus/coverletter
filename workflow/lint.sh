#!/bin/sh
# Check the work-item invariants that the workflow rules assume.
#
#   sh workflow/lint.sh [project-dir]
#
# Exit status is 0 when everything holds, 1 when it does not, 2 on misuse.
# One line per problem, so it can gate a commit or a CI job unread.

set -u

ROOT=${1:-.}
if [ ! -d "$ROOT" ]; then
    printf 'lint: not a directory: %s\n' "$ROOT" >&2
    exit 2
fi
cd "$ROOT" || exit 2

errors=0

err() {
    printf '%s: %s\n' "$1" "$2" >&2
    errors=$((errors + 1))
}

TMP=${TMPDIR:-/tmp}/workflow-lint.$$
trap 'rm -f "$TMP".*' 0 1 2 15

# Required structure.

for d in issues tasks/open tasks/closed features workflow; do
    if [ ! -d "$d" ]; then
        err "$d" "missing directory"
    fi
done

# Collect work items.

: >"$TMP.items"
for d in issues tasks/open tasks/closed; do
    if [ ! -d "$d" ]; then
        continue
    fi
    for f in "$d"/*.md; do
        if [ -e "$f" ]; then
            printf '%s\n' "$f" >>"$TMP.items"
        fi
    done
done

# Filenames must be unique across the three lifecycle directories.

: >"$TMP.names"
while IFS= read -r f; do
    printf '%s|%s\n' "$(basename "$f")" "$f" >>"$TMP.names"
done <"$TMP.items"
sort "$TMP.names" >"$TMP.sorted"
prev=''
prev_path=''
while IFS='|' read -r name path; do
    if [ "$name" = "$prev" ]; then
        err "$path" "duplicate work-item filename (also $prev_path)"
    fi
    prev=$name
    prev_path=$path
done <"$TMP.sorted"

# Per-item checks.

while IFS= read -r f; do
    title=$(awk '
        BEGIN { fm = 0; seen = 0 }
        NR == 1 && $0 == "---" { fm = 1; next }
        fm && $0 == "---" { fm = 0; next }
        fm { next }
        !seen && NF > 0 { print; seen = 1 }
    ' "$f")
    case $title in
    "# "*) ;;
    *) err "$f" "no '# ' title after front matter" ;;
    esac

    awk '
        BEGIN { fm = 0; block = 0 }
        NR == 1 { if ($0 == "---") { fm = 1; next } else { exit } }
        fm && $0 == "---" { exit }
        fm && /^dependencies:[[:space:]]*\[/ {
            s = $0
            sub(/^dependencies:[[:space:]]*\[/, "", s)
            sub(/\].*$/, "", s)
            n = split(s, parts, ",")
            for (i = 1; i <= n; i++) {
                t = parts[i]
                gsub(/[[:space:]]/, "", t)
                if (t != "") print t
            }
            next
        }
        fm && /^dependencies:[[:space:]]*$/ { block = 1; next }
        block && /^[[:space:]]*-[[:space:]]*/ {
            t = $0
            sub(/^[[:space:]]*-[[:space:]]*/, "", t)
            gsub(/[[:space:]]/, "", t)
            if (t != "") print t
            next
        }
        block { block = 0 }
    ' "$f" | sed "s/['\"]//g" >"$TMP.deps"

    while IFS= read -r dep; do
        if [ -z "$dep" ]; then
            continue
        fi
        found=0
        for d in issues tasks/open tasks/closed; do
            if [ -f "$d/$dep" ]; then
                found=1
            fi
        done
        if [ "$found" -eq 0 ]; then
            err "$f" "dependency does not resolve: $dep (expected a bare work-item filename)"
        fi
    done <"$TMP.deps"

    case $f in
    tasks/closed/*)
        if ! grep -q '^## Outcome[[:space:]]*$' "$f"; then
            err "$f" "closed task has no '## Outcome' section"
        fi
        ;;
    esac
done <"$TMP.items"

# Relative links between markdown files must resolve.

find . -name '*.md' \
    -not -path './.git/*' \
    -not -path './node_modules/*' \
    -not -path './vendor/*' \
    -not -path './dist/*' >"$TMP.md" 2>/dev/null

while IFS= read -r f; do
    dir=$(dirname "$f")
    grep -o '](\([^)]*\))' "$f" 2>/dev/null | sed 's/^](//; s/)$//' >"$TMP.links"
    while IFS= read -r link; do
        case $link in
        '' | '#'* | /* | http://* | https://* | mailto:* | '//'*) continue ;;
        esac
        target=${link%%#*}
        if [ -z "$target" ]; then
            continue
        fi
        if [ ! -e "$dir/$target" ]; then
            err "$f" "link target does not resolve: $link"
        fi
    done <"$TMP.links"
done <"$TMP.md"

if [ "$errors" -gt 0 ]; then
    printf '\n%d problem(s).\n' "$errors" >&2
    exit 1
fi

printf 'lint: ok\n'
exit 0
