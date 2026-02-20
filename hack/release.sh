#!/usr/bin/env bash

# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2026-02-20T18:04:59Z by kres dc032d7.

set -e

RELEASE_TOOL_IMAGE="ghcr.io/siderolabs/release-tool:latest"

function release-tool {
  docker pull "${RELEASE_TOOL_IMAGE}" >/dev/null
  docker run --rm -w /src -v "${PWD}":/src:ro "${RELEASE_TOOL_IMAGE}" -l -d -n -t "${1}" ./hack/release.toml
}

function changelog {
  if [ "$#" -eq 1 ]; then
    (release-tool ${1}; echo; cat CHANGELOG.md) > CHANGELOG.md- && mv CHANGELOG.md- CHANGELOG.md
  else
    echo 1>&2 "Usage: $0 changelog [tag]"
    exit 1
  fi
}

function release-notes {
  release-tool "${2}" > "${1}"
}

function cherry-pick {
  if [ $# -ne 2 ]; then
    echo 1>&2 "Usage: $0 cherry-pick <commit> <branch>"
    exit 1
  fi

  git checkout $2
  git fetch
  git rebase upstream/$2
  git cherry-pick -x $1
}

function commit {
  if [ $# -ne 1 ]; then
    echo 1>&2 "Usage: $0 commit <tag>"
    exit 1
  fi

  if is_on_main_branch; then
    update_license_files
  fi

  git commit -s -m "release($1): prepare release" -m "This is the official $1 release."
}

function is_on_main_branch {
  main_remotes=("upstream" "origin")
  branch_names=("main" "master")
  current_branch=$(git rev-parse --abbrev-ref HEAD)

  echo "Check current branch: $current_branch"

  for remote in "${main_remotes[@]}"; do
    echo "Fetch remote $remote..."

    if ! git fetch --quiet "$remote" &>/dev/null; then
      echo "Failed to fetch $remote, skip..."

      continue
    fi

    for branch_name in "${branch_names[@]}"; do
      if ! git rev-parse --verify "$branch_name" &>/dev/null; then
        echo "Branch $branch_name does not exist, skip..."

        continue
      fi

      echo "Branch $remote/$branch_name exists, comparing..."

      merge_base=$(git merge-base "$current_branch" "$remote/$branch_name")
      latest_main=$(git rev-parse "$remote/$branch_name")

      if [ "$merge_base" = "$latest_main" ]; then
        echo "Current branch is up-to-date with $remote/$branch_name"

        return 0
      else
        echo "Current branch is not on $remote/$branch_name"

        return 1
      fi
    done
  done

  echo "No main or master branch found on any remote"

  return 1
}

function update_license_files {
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  parent_dir="$(dirname "$script_dir")"
  current_year=$(date +"%Y")
  change_date=$(date -v+4y +"%Y-%m-%d" 2>/dev/null || date -d "+4 years" +"%Y-%m-%d" 2>/dev/null || date --date="+4 years" +"%Y-%m-%d")

  # Find LICENSE and .kres.yaml files recursively in the parent directory (project root)
  find "$parent_dir" \( -name "LICENSE" -o -name ".kres.yaml" \) -type f | while read -r file; do
    temp_file="${file}.tmp"

    if [[ $file == *"LICENSE" ]]; then
      if grep -q "^Business Source License" "$file"; then
        sed -e "s/The Licensed Work is (c) [0-9]\{4\}/The Licensed Work is (c) $current_year/" \
          -e "s/Change Date:          [0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}/Change Date:          $change_date/" \
          "$file" >"$temp_file"
      else
        continue # Not a Business Source License file
      fi
    elif [[ $file == *".kres.yaml" ]]; then
      sed -E 's/^([[:space:]]*)ChangeDate:.*$/\1ChangeDate: "'"$change_date"'"/' "$file" >"$temp_file"
    fi

    # Check if the file has changed
    if ! cmp -s "$file" "$temp_file"; then
      mv "$temp_file" "$file"
      echo "Updated: $file"
      git add "$file"
    else
      echo "No changes: $file"
      rm "$temp_file"
    fi
  done
}

if declare -f "$1" > /dev/null
then
  cmd="$1"
  shift
  $cmd "$@"
else
  cat <<EOF
Usage:
  commit:        Create the official release commit message (updates BUSL license dates if there is any).
  cherry-pick:   Cherry-pick a commit into a release branch.
  changelog:     Update the specified CHANGELOG.
  release-notes: Create release notes for GitHub release.
EOF

  exit 1
fi

