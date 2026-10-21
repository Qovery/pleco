#!/usr/bin/env bash
#
# find-gcp-orphans.sh — identify GCP resources whose Qovery GKE cluster no longer exists.
#
# Identification only: nothing is deleted.
#
# A resource is reported as ORPHAN when:
#   1. its tags (description JSON or labels) contain a non-empty `cluster_id`
#   2. no live GKE cluster matches that id (by `cluster_id` label or name substring)
#   3. its `creation_date` is older than the grace period (default 6h)
#   4. it is not protected (ttl=0 means protected -> skipped)
#
# Resources matching 1+2 but younger than the grace period are reported as
# RECENT (possible provisioning in progress). Resources matching 1+2 with no
# creation_date are reported as ORPHAN? (age unverifiable).
#
# Usage:
#   ./find-gcp-orphans.sh [PROJECT_ID]
#
# Environment:
#   GRACE_HOURS   grace period in hours (default: 6)
#
# Requires: gcloud (authenticated), jq

set -euo pipefail

PROJECT="${1:-qovery-gcp-tests}"
GRACE_HOURS="${GRACE_HOURS:-6}"

for bin in gcloud jq; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "ERROR: required binary '$bin' not found in PATH" >&2
    exit 1
  fi
done

NOW="$(date +%s)"
GRACE_SECONDS=$((GRACE_HOURS * 3600))

ORPHAN_COUNT=0
RECENT_COUNT=0
UNKNOWN_AGE_COUNT=0
PROTECTED_COUNT=0

echo "Project:      $PROJECT"
echo "Grace period: ${GRACE_HOURS}h"
echo

# ---------------------------------------------------------------------------
# Live GKE clusters (all locations). Abort on failure: without the live set,
# every resource would look orphaned.
# ---------------------------------------------------------------------------
echo "==> Listing live GKE clusters (all locations)..."
if ! CLUSTERS_JSON="$(gcloud container clusters list --project "$PROJECT" --format=json 2>/dev/null)"; then
  echo "ERROR: failed to list GKE clusters — aborting for safety" >&2
  exit 1
fi

LIVE_LABEL_IDS="$(jq -r '.[].resourceLabels.cluster_id // empty' <<<"$CLUSTERS_JSON")"
LIVE_NAMES="$(jq -r '.[].name' <<<"$CLUSTERS_JSON")"

CLUSTER_TOTAL="$(jq -r 'length' <<<"$CLUSTERS_JSON")"
echo "    $CLUSTER_TOTAL live cluster(s) found"
if [ "$CLUSTER_TOTAL" -gt 0 ]; then
  jq -r '.[] | "      - \(.name) (\(.location)) cluster_id=\(.resourceLabels.cluster_id // "-") status=\(.status)"' <<<"$CLUSTERS_JSON"
fi
echo

# Returns 0 if a live cluster matches the given cluster_id.
is_live() {
  local id="$1"
  if [ -n "$LIVE_LABEL_IDS" ] && grep -qxF "$id" <<<"$LIVE_LABEL_IDS"; then
    return 0
  fi
  if [ -n "$LIVE_NAMES" ] && grep -qF "$id" <<<"$LIVE_NAMES"; then
    return 0
  fi
  return 1
}

# classify KIND NAME CLUSTER_ID CREATION_EPOCH TTL
# CREATION_EPOCH and TTL may be empty strings.
classify() {
  local kind="$1" name="$2" cluster_id="$3" creation="$4" ttl="$5"

  [ -z "$cluster_id" ] && return 0

  if is_live "$cluster_id"; then
    return 0
  fi

  if [ "$ttl" = "0" ]; then
    PROTECTED_COUNT=$((PROTECTED_COUNT + 1))
    printf '  PROTECTED  %-22s %-55s cluster_id=%-12s (ttl=0, never deleted)\n' "$kind" "$name" "$cluster_id"
    return 0
  fi

  if [ -z "$creation" ]; then
    UNKNOWN_AGE_COUNT=$((UNKNOWN_AGE_COUNT + 1))
    printf '  ORPHAN?    %-22s %-55s cluster_id=%-12s (no creation_date, age unverifiable)\n' "$kind" "$name" "$cluster_id"
    return 0
  fi

  local age=$((NOW - creation))
  if [ "$age" -lt "$GRACE_SECONDS" ]; then
    RECENT_COUNT=$((RECENT_COUNT + 1))
    printf '  RECENT     %-22s %-55s cluster_id=%-12s (age %dm < grace %dh, maybe provisioning)\n' "$kind" "$name" "$cluster_id" "$((age / 60))" "$GRACE_HOURS"
    return 0
  fi

  ORPHAN_COUNT=$((ORPHAN_COUNT + 1))
  printf '  ORPHAN     %-22s %-55s cluster_id=%-12s age=%dh\n' "$kind" "$name" "$cluster_id" "$((age / 3600))"
}

# scan KIND TSV_STREAM — TSV columns: name, cluster_id, creation_epoch, ttl
scan() {
  local kind="$1" tsv="$2"
  local name cluster_id creation ttl
  echo "==> Scanning ${kind}..."
  if [ -z "$tsv" ]; then
    echo "    none with cluster_id tag"
    echo
    return 0
  fi
  while IFS=$'\t' read -r name cluster_id creation ttl; do
    classify "$kind" "$name" "$cluster_id" "$creation" "$ttl"
  done <<<"$tsv"
  echo
}

# Shared jq defs:
#   name_id   — extract Qovery cluster short id (z + 8 hex) from a resource
#               name/email; only for qovery-named resources, takes last match
#               (e.g. rm-bucket-qovery-logs-z0dab9f8f -> z0dab9f8f)
#   iso_epoch — ISO-8601 timestamp -> unix epoch string ("" if unparsable)
JQ_DEFS='
  def name_id:
    if (. // "" | test("qovery"))
    then ([match("z[0-9a-f]{8}\\b"; "g").string] | last // "")
    else "" end;
  def iso_epoch:
    (. // "") | if . == "" then ""
    else (sub("\\.[0-9]+"; "") | sub("\\+00:00$"; "Z")
          | (try fromdateiso8601 catch "") | tostring)
    end;
'

# jq helper for resources tagged via a JSON blob in .description
# emits: name \t cluster_id \t creation_date \t ttl
JQ_DESC_TAGS="$JQ_DEFS"'.[]
  | . as $r
  | ($r.description // "" | fromjson? // {}) as $tags
  | (($tags.cluster_id // "" | tostring) | if . == "" then ($r.name | name_id) else . end) as $id
  | select($id != "")
  | [$r.name, $id, ($tags.creation_date // "" | tostring), ($tags.ttl // "" | tostring)]
  | @tsv'

# ---------------------------------------------------------------------------
# VPC networks (description JSON)
# ---------------------------------------------------------------------------
NETWORKS="$(gcloud compute networks list --project "$PROJECT" --format=json | jq -r "$JQ_DESC_TAGS")" || NETWORKS=""
scan "vpc-network" "$NETWORKS"

# ---------------------------------------------------------------------------
# Cloud Routers, all regions (description JSON)
# ---------------------------------------------------------------------------
ROUTERS="$(gcloud compute routers list --project "$PROJECT" --format=json | jq -r "$JQ_DESC_TAGS")" || ROUTERS=""
scan "cloud-router" "$ROUTERS"

# ---------------------------------------------------------------------------
# IAM service accounts (description JSON, fallback: id in email local part,
# e.g. thanos-qovery-z8004ce8b@...). List API returns no creation time, so
# name-matched SAs have unverifiable age (reported as ORPHAN?).
# ---------------------------------------------------------------------------
SAS="$(gcloud iam service-accounts list --project "$PROJECT" --format=json \
  | jq -r "$JQ_DEFS"'.[]
      | . as $r
      | ($r.description // "" | fromjson? // {}) as $tags
      | (($tags.cluster_id // "" | tostring) | if . == "" then ($r.email | name_id) else . end) as $id
      | select($id != "")
      | [$r.email, $id, ($tags.creation_date // "" | tostring), ($tags.ttl // "" | tostring)]
      | @tsv')" || SAS=""
scan "iam-service-account" "$SAS"

# ---------------------------------------------------------------------------
# Storage buckets (labels, fallback: id in name, e.g. qovery-logs-ze2a47575;
# creation falls back to the bucket's real creation timestamp)
# ---------------------------------------------------------------------------
BUCKETS="$(gcloud storage buckets list --project "$PROJECT" --format=json \
  | jq -r "$JQ_DEFS"'.[]
      | . as $r
      | (($r.labels // $r.metadata.labels) // {}) as $labels
      | (($labels.cluster_id // "" | tostring) | if . == "" then ($r.name | name_id) else . end) as $id
      | select($id != "")
      | [$r.name, $id,
         (($labels.creation_date // "" | tostring)
          | if . == "" then (($r.timeCreated // $r.metadata.timeCreated // $r.creation_time // "") | iso_epoch) else . end),
         ($labels.ttl // "" | tostring)]
      | @tsv')" || BUCKETS=""
scan "storage-bucket" "$BUCKETS"

# ---------------------------------------------------------------------------
# Artifact Registry repositories, all locations (labels; cluster_id label may
# hold the full cluster name `qovery-zXXXXXXXX` — live-name matching covers it.
# Fallback: id in repo name; creation falls back to repo createTime)
# ---------------------------------------------------------------------------
REPOS="$(gcloud artifacts repositories list --project "$PROJECT" --format=json 2>/dev/null \
  | jq -r "$JQ_DEFS"'.[]
      | . as $r
      | ($r.labels // {}) as $labels
      | (($labels.cluster_id // "" | tostring) | if . == "" then ($r.name | name_id) else . end) as $id
      | select($id != "")
      | [$r.name, $id,
         (($labels.creation_date // "" | tostring)
          | if . == "" then (($r.createTime // "") | iso_epoch) else . end),
         ($labels.ttl // "" | tostring)]
      | @tsv')" || REPOS=""
scan "artifact-registry" "$REPOS"

# ---------------------------------------------------------------------------
# Cloud Run jobs, all regions (labels, fallback: id in name, e.g.
# rm-bucket-qovery-logs-z0dab9f8f; creation falls back to job creation time)
# ---------------------------------------------------------------------------
JOBS="$(gcloud run jobs list --project "$PROJECT" --format=json 2>/dev/null \
  | jq -r "$JQ_DEFS"'.[]
      | . as $r
      | (($r.labels // $r.metadata.labels) // {}) as $labels
      | (($r.metadata.name // $r.name) // "") as $name
      | (($labels.cluster_id // "" | tostring) | if . == "" then ($name | name_id) else . end) as $id
      | select($id != "")
      | [$name, $id,
         (($labels.creation_date // "" | tostring)
          | if . == "" then (($r.metadata.creationTimestamp // $r.createTime // "") | iso_epoch) else . end),
         ($labels.ttl // "" | tostring)]
      | @tsv')" || JOBS=""
scan "cloud-run-job" "$JOBS"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo "=================================================================="
echo "Summary"
echo "  ORPHAN     (cluster gone, older than grace):  $ORPHAN_COUNT"
echo "  ORPHAN?    (cluster gone, age unverifiable):  $UNKNOWN_AGE_COUNT"
echo "  RECENT     (cluster gone, within grace):      $RECENT_COUNT"
echo "  PROTECTED  (cluster gone, ttl=0):             $PROTECTED_COUNT"
echo "=================================================================="
echo
echo "Nothing was deleted. Review ORPHAN entries before any cleanup."
