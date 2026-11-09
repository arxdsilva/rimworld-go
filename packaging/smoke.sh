# shellcheck shell=bash
# Shared by the installers' smoke tests (scripts/package_macos.sh,
# scripts/package_linux.sh); source it.

# check_inside fails when an asset line of a -selfcheck report points
# outside root (the game would have found some other copy of its files,
# such as the repository's, instead of the installed ones).
check_inside() {
  local report=$1 root=$2 line p n=0
  while IFS= read -r line; do
    case $line in
      "ok       translations: "* | "ok       art: "* | "ok       menu splash: "* | "ok       sounds: "*)
        p=${line#*: }
        case $p in
          "$root" | "$root"/*) n=$((n + 1)) ;;
          *) echo "smoke: '$line' is outside $root" >&2; return 1 ;;
        esac
        ;;
    esac
  done <<< "$report"
  if [ "$n" -ne 4 ]; then
    echo "smoke: the -selfcheck report names $n of the 4 asset roots" >&2
    return 1
  fi
}
