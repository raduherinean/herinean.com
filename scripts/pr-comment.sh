#!/usr/bin/env bash
# One sticky CI comment per pull request (marker <!-- herinean-ci -->): create or update. usage: pr-comment.sh PR FILE | pr-comment.sh PR --get
set -euo pipefail
pr=${1:?pr}; src=${2:?file or --get}
repo=${GITHUB_REPOSITORY:?}
id=$(gh api "repos/$repo/issues/$pr/comments" --paginate --jq '.[] | select(.body | startswith("<!-- herinean-ci -->")) | .id' | sed -n 1p)
if [ "$src" = "--get" ]; then [ -n "$id" ] && gh api "repos/$repo/issues/comments/$id" --jq .body || echo "<!-- herinean-ci -->"; exit 0; fi
if [ -n "$id" ]; then gh api -X PATCH "repos/$repo/issues/comments/$id" -F body=@"$src" >/dev/null
else gh api -X POST "repos/$repo/issues/$pr/comments" -F body=@"$src" >/dev/null; fi
