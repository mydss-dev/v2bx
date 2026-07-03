#!/usr/bin/env bash
set -e

src="https://raw.githubusercontent.com/wyx2685/V2bX-script/master/V2bX.sh"
repo="mydss-dev/v2bx"
branch="main"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$src" -o "$tmp"
else
  wget -qO "$tmp" "$src"
fi

sed -i \
  -e "s#https://raw.githubusercontent.com/wyx2685/V2bX-script/master/install.sh#https://raw.githubusercontent.com/${repo}/${branch}/install.sh#g" \
  -e "s#https://raw.githubusercontent.com/wyx2685/V2bX-script/master/V2bX.sh#https://raw.githubusercontent.com/${repo}/${branch}/V2bX.sh#g" \
  -e "s#https://raw.githubusercontent.com/wyx2685/V2bX-script/master/initconfig.sh#https://raw.githubusercontent.com/${repo}/${branch}/initconfig.sh#g" \
  "$tmp"

/bin/bash "$tmp" "$@"
