#!/usr/bin/env bash
# Pleco release, in two steps:
#   bump: push a branch setting the new version everywhere, to merge through an MR
#   tag:  once merged, tag main with that version and push the tag (GitLab builds the image, deploys test clusters)
set -euo pipefail

REMOTE="origin"
RELEASE_BRANCH="main"
PROJECT_URL="https://gitlab.com/qovery/backend/pleco"

usage() {
  cat <<'EOF'
Usage: scripts/release.sh bump [X.Y.Z] [--yes] [--dry-run]
       scripts/release.sh tag [--yes] [--dry-run]

  bump   Branch from main, set the version in cmd/version.go and the chart, push the branch.
         Default version: patch bump of the latest vX.Y.Z tag.
  tag    Tag main with the version its files declare, then push the tag.

  --yes      skip the confirmation prompt
  --dry-run  run every check and print the summary, change nothing
EOF
}

die() {
  echo "Error: $*" >&2
  exit 1
}

is_version() {
  echo "${1:-}" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'
}

# X.Y.Z -> sortable integer, base 10 forced (1.08.0 is not octal)
ver_num() {
  local major minor patch
  IFS='.' read -r major minor patch <<< "$1"
  echo $(( 10#$major * 10000000000 + 10#$minor * 100000 + 10#$patch ))
}

# Highest vX.Y.Z tag without its v. Non-release tags (v1.2-rc1) are dropped: they break ver_num.
latest_release_version() {
  local all tags
  if ! all="$(git for-each-ref --sort=-v:refname --format='%(refname:strip=2)' 'refs/tags/v*')"; then
    return 1
  fi
  tags="$(printf '%s\n' "$all" | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' || true)"
  tags="${tags%%$'\n'*}"
  echo "${tags#v}"
}

require_in_sync() {
  git fetch --quiet --tags "$REMOTE"
  if [ "$(git rev-parse HEAD)" != "$(git rev-parse "$REMOTE/$RELEASE_BRANCH")" ]; then
    die "${1:-local $RELEASE_BRANCH differs from $REMOTE/$RELEASE_BRANCH. Pull or push first.}"
  fi
}

require_clean_release_branch() {
  local branch
  branch="$(git rev-parse --abbrev-ref HEAD)"
  [ "$branch" = "$RELEASE_BRANCH" ] || die "on branch '$branch', releases start from '$RELEASE_BRANCH'."
  [ -z "$(git status --porcelain)" ] || die "working tree not clean. Commit or stash first."
  echo "==> Fetching $REMOTE"
  require_in_sync
}

# Places holding the version, as "label|file|line pattern"
VERSION_PLACES=(
  "cmd/version.go|cmd/version.go|// ci-version-check"
  "Chart.yaml version|charts/pleco/Chart.yaml|^version:"
  "Chart.yaml appVersion|charts/pleco/Chart.yaml|^appVersion:"
  "values.yaml plecoImageTag|charts/pleco/values.yaml|^[[:space:]]*plecoImageTag:"
)

# Version at one place, empty when missing
read_version() {
  local file pattern
  IFS='|' read -r _ file pattern <<< "$1"
  grep -E "$pattern" "$file" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true
}

print_declared_versions() {
  local place
  for place in "${VERSION_PLACES[@]}"; do
    printf '  %-26s : %s\n' "${place%%|*}" "$(read_version "$place" | grep . || echo missing)"
  done
}

# Version declared at every place, empty when they differ or one is missing
declared_version() {
  local place version first=""
  for place in "${VERSION_PLACES[@]}"; do
    version="$(read_version "$place")"
    if [ -z "$version" ] || { [ -n "$first" ] && [ "$version" != "$first" ]; }; then
      return 0
    fi
    first="$version"
  done
  echo "$first"
}

sed_in_place() {
  sed -E -i.bak "$1" "$2"
  rm -f "$2.bak"
}

set_version() {
  sed_in_place "s#\"[0-9]+\.[0-9]+\.[0-9]+\"( // ci-version-check)#\"$1\"\1#" cmd/version.go
  sed_in_place "s#^(version|appVersion):.*#\1: $1#" charts/pleco/Chart.yaml
  sed_in_place "s#^([[:space:]]*plecoImageTag:).*#\1 \"$1\"#" charts/pleco/values.yaml
}

confirm() {
  if [ "$YES" = "true" ]; then
    return 0
  fi
  [ -t 0 ] || die "not a TTY, rerun with --yes to skip confirmation."
  printf '  %s [y/N] ' "$1"
  read -r answer || answer=""
  case "$answer" in
    y|Y|yes|YES) ;;
    *) echo "Aborted, nothing changed."; exit 1 ;;
  esac
}

stop_if_dry_run() {
  if [ "$DRY_RUN" = "true" ]; then
    echo "Dry run: nothing changed."
    exit 0
  fi
}

latest_or_die() {
  local version
  version="$(latest_release_version)" || die "listing tags failed (git error above)."
  [ -n "$version" ] || die "no vX.Y.Z tag found."
  echo "$version"
}

check_new_version() {
  local new="$1" current="$2"
  is_version "$new" || die "'$new' is not an X.Y.Z version."
  [ "$(ver_num "$new")" -gt "$(ver_num "$current")" ] || die "$new is not greater than latest tag v$current."
  if git rev-parse -q --verify "refs/tags/v$new" >/dev/null; then
    die "tag v$new already exists."
  fi
}

commits_since() {
  local commits
  commits="$(git log --oneline "v$1..HEAD")"
  [ -n "$commits" ] || die "no commit since v$1, nothing to release."
  echo "$commits"
}

print_commits() {
  echo "  $(printf '%s\n' "$2" | wc -l | tr -d ' ') commit(s) since v$1:"
  printf '%s\n' "$2" | sed 's/^/    /'
}

cmd_bump() {
  require_clean_release_branch
  local current new branch commits
  current="$(latest_or_die)"
  new="${VERSION:-}"
  if [ -z "$new" ]; then
    local major minor patch
    IFS='.' read -r major minor patch <<< "$current"
    new="$major.$minor.$((patch + 1))"
  fi
  check_new_version "$new" "$current"
  commits="$(commits_since "$current")"

  local declared
  declared="$(declared_version)"
  if [ -n "$declared" ] && [ "$(ver_num "$declared")" -gt "$(ver_num "$current")" ]; then
    die "$RELEASE_BRANCH already declares $declared, not tagged yet. Run 'task release' to tag it."
  fi

  branch="chore/bump-$new"
  if git rev-parse -q --verify "refs/heads/$branch" >/dev/null; then
    die "local branch $branch already exists."
  fi
  local remote_rc=0
  git ls-remote --exit-code --heads "$REMOTE" "$branch" >/dev/null || remote_rc=$?
  case "$remote_rc" in
    0) die "branch $branch already exists on $REMOTE." ;;
    2) ;;
    *) die "can't list branches on $REMOTE (git error above)." ;;
  esac

  echo
  echo "  latest tag  : v$current"
  echo "  new version : $new"
  echo "  branch      : $branch (from $(git log -1 --oneline HEAD))"
  echo
  echo "  Version today:"
  print_declared_versions
  echo
  print_commits "$current" "$commits"
  echo
  stop_if_dry_run
  confirm "Set version $new on branch $branch and push it?"
  require_in_sync "$REMOTE/$RELEASE_BRANCH moved meanwhile, nothing changed. Pull and re-run."

  git switch --quiet -c "$branch"
  if ! { set_version "$new" &&
    [ "$(declared_version)" = "$new" ] &&
    git add cmd/version.go charts/pleco/Chart.yaml charts/pleco/values.yaml &&
    git commit --quiet -m "chore(release): bump to $new" &&
    git push --quiet -u "$REMOTE" "$branch"; }; then
    git switch --quiet --discard-changes "$RELEASE_BRANCH"
    git branch --quiet -D "$branch"
    die "bump to $new failed (error above). Back on $RELEASE_BRANCH, local branch $branch removed, nothing pushed."
  fi
  git switch --quiet "$RELEASE_BRANCH"

  echo
  echo "Pushed $branch. Open and merge its MR:"
  echo "  $PROJECT_URL/-/merge_requests/new?merge_request%5Bsource_branch%5D=${branch//\//%2F}&merge_request%5Btarget_branch%5D=$RELEASE_BRANCH"
  echo "Then: git pull && task release"
}

cmd_tag() {
  require_clean_release_branch
  local current new commits latest
  current="$(latest_or_die)"
  [ -z "$VERSION" ] || die "tag takes no version: it tags the version $RELEASE_BRANCH declares."
  new="$(declared_version)"
  if [ -z "$new" ]; then
    print_declared_versions >&2
    die "$RELEASE_BRANCH does not declare one version everywhere. Run 'task release:bump' and merge its MR first."
  fi
  check_new_version "$new" "$current"
  commits="$(commits_since "$current")"

  echo
  echo "  latest tag  : v$current"
  echo "  new tag     : v$new"
  echo "  commit      : $(git log -1 --oneline HEAD)"
  echo
  print_commits "$current" "$commits"
  echo
  echo "  Pushing v$new starts the GitLab tag pipeline:"
  echo "    image public.ecr.aws/r3m4q3r9/pleco:$new (amd64, arm64),"
  echo "    then helm upgrade of Pleco on the Scaleway, GCP, EKS, AWS sandbox and Azure test clusters."
  echo
  stop_if_dry_run
  confirm "Create and push tag v$new (latest: v$current)?"

  # main, the tag or the latest release may have moved while the prompt was open
  require_in_sync "$REMOTE/$RELEASE_BRANCH moved meanwhile, nothing pushed. Re-run to release the new HEAD."
  if git rev-parse -q --verify "refs/tags/v$new" >/dev/null; then
    die "tag v$new appeared on $REMOTE meanwhile, nothing pushed."
  fi
  latest="$(latest_or_die)"
  [ "$(ver_num "$new")" -gt "$(ver_num "$latest")" ] || die "v$latest was released meanwhile, v$new would go backwards. Nothing pushed."

  git tag -a "v$new" -m "v$new"
  if ! git push "$REMOTE" "v$new"; then
    git tag -d "v$new" >/dev/null
    die "pushing v$new failed (error above). Local tag removed, nothing released."
  fi

  echo
  echo "Pushed v$new. Tag pipeline:"
  echo "  $PROJECT_URL/-/pipelines?scope=tags&ref=v$new"
}

command="${1:-}"
[ $# -gt 0 ] && shift
VERSION=""
YES="false"
DRY_RUN="false"
for arg in "$@"; do
  case "$arg" in
    -y|--yes) YES="true" ;;
    --dry-run) DRY_RUN="true" ;;
    -h|--help) usage; exit 0 ;;
    -*) usage >&2; die "unknown flag $arg" ;;
    *) [ -z "$VERSION" ] || die "only one version allowed"; VERSION="${arg#v}" ;;
  esac
done

cd "$(git rev-parse --show-toplevel)"
case "$command" in
  bump) cmd_bump ;;
  tag) cmd_tag ;;
  -h|--help|help) usage ;;
  *) usage >&2; exit 1 ;;
esac
