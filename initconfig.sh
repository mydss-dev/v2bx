#!/usr/bin/env bash

src="https://raw.githubusercontent.com/wyx2685/V2bX-script/master/initconfig.sh"
tmp="$(mktemp)"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$src" -o "$tmp"
else
  wget -qO "$tmp" "$src"
fi

# This file is sourced by install.sh, so source the official init script here.
# shellcheck source=/dev/null
source "$tmp"
rm -f "$tmp"
