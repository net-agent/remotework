#!/usr/bin/env bash
# 依据 git 提交历史生成 GitHub Release 说明。
#
# 不依赖 PR 元数据：直接列举 <previous-tag>..<tag> 的非合并提交，按
# conventional-commit 前缀分组，保证直接推送（未走 PR）的提交也能完整呈现。
#
# 用法: release-notes.sh <tag> [previous-tag]
#   未指定 previous-tag 时，自动取 <tag> 之前最近的 tag。
#   输出为 Markdown，可直接作为 softprops/action-gh-release 的 body_path。
set -euo pipefail

tag="${1:?usage: release-notes.sh <tag> [previous-tag]}"
repo="${GITHUB_REPOSITORY:-net-agent/remotework}"
prev="${2:-}"

if [ -z "$prev" ]; then
  # <tag>^ 保证不会把当前 tag 自身当成"上一个 tag"
  prev="$(git describe --tags --abbrev=0 "${tag}^" 2>/dev/null || true)"
fi

if [ -n "$prev" ]; then
  range="${prev}..${tag}"
  echo "## Changes since ${prev}"
else
  range="${tag}"
  echo "## Changes"
fi
echo

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

count=0
# bash 3.2 对 [[ =~ ]] 内联正则的解析有缺陷，先存到变量里
subject_re='^([A-Za-z]+)(\([^)]*\))?!?:'
while IFS=$'\t' read -r sha subj; do
  [ -n "$sha" ] || continue
  count=$((count + 1))

  type="other"
  breaking=""
  if [[ "$subj" =~ $subject_re ]]; then
    type="$(printf '%s' "${BASH_REMATCH[1]}" | tr '[:upper:]' '[:lower:]')"
    case "${BASH_REMATCH[0]}" in
      *'!:'*) breaking="**BREAKING** " ;;
    esac
  fi
  case "$type" in
    feat|fix|perf|refactor|docs|test|ci|build|chore|revert) ;;
    *) type="other" ;;
  esac

  printf -- '- %s%s (`%s`)\n' "$breaking" "$subj" "$sha" >> "$tmp/$type"
done < <(git log --no-merges --pretty=tformat:'%h%x09%s' "$range")

if [ "$count" -eq 0 ]; then
  echo "_No commits in this range._"
  echo
fi

group_title() {
  case "$1" in
    feat) echo "Features" ;;
    fix) echo "Fixes" ;;
    perf) echo "Performance" ;;
    refactor) echo "Refactoring" ;;
    docs) echo "Documentation" ;;
    test) echo "Tests" ;;
    ci) echo "CI / Build" ;;
    build) echo "CI / Build" ;;
    chore) echo "Chores" ;;
    revert) echo "Reverts" ;;
    *) echo "Other Changes" ;;
  esac
}

for type in feat fix perf refactor docs test ci build chore revert other; do
  [ -s "$tmp/$type" ] || continue
  echo "### $(group_title "$type")"
  echo
  cat "$tmp/$type"
  echo
done

if [ -n "$prev" ]; then
  echo "**Full Changelog**: https://github.com/${repo}/compare/${prev}...${tag}"
fi
