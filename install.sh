#!/usr/bin/env bash
set -e

OFFICIAL_INSTALL="https://raw.githubusercontent.com/wyx2685/V2bX-script/master/install.sh"
SELF_REPO="mydss-dev/v2bx"
SELF_BRANCH="main"
TMP_INSTALL="$(mktemp)"

cleanup() {
  rm -f "$TMP_INSTALL"
}
trap cleanup EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$OFFICIAL_INSTALL" -o "$TMP_INSTALL"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$TMP_INSTALL" "$OFFICIAL_INSTALL"
else
  echo "需要 curl 或 wget 才能下载安装脚本"
  exit 1
fi

# Keep the official installer logic, only replace sources to this repository.
sed -i \
  -e "s#https://api.github.com/repos/wyx2685/V2bX/releases/latest#https://api.github.com/repos/${SELF_REPO}/releases/latest#g" \
  -e "s#https://github.com/wyx2685/V2bX/releases/download#https://github.com/${SELF_REPO}/releases/download#g" \
  -e "s#https://raw.githubusercontent.com/wyx2685/V2bX-script/master/V2bX.sh#https://raw.githubusercontent.com/${SELF_REPO}/${SELF_BRANCH}/V2bX.sh#g" \
  -e "s#https://raw.githubusercontent.com/wyx2685/V2bX-script/master/initconfig.sh#https://raw.githubusercontent.com/${SELF_REPO}/${SELF_BRANCH}/initconfig.sh#g" \
  "$TMP_INSTALL"

bash "$TMP_INSTALL" "$@"
