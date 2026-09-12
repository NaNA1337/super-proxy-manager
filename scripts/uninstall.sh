#!/usr/bin/env bash
set -euo pipefail

readonly service_name="super-proxy-web.service"
readonly service_file="/etc/systemd/system/${service_name}"
readonly service_dropins="/etc/systemd/system/${service_name}.d"
readonly binary_file="/usr/local/bin/super-proxy-web"
readonly data_root="/var/lib/super-proxy-manager"

purge_data=false
assume_yes=false

usage() {
    cat <<'EOF'
Usage: uninstall.sh [--purge-data] [--yes]

Remove Super-Proxy Manager from this server.

  --purge-data  Also delete /var/lib/super-proxy-manager permanently
  --yes         Skip the confirmation required by --purge-data
  -h, --help    Show this help

By default, the service and binary are removed while Manager data is preserved.
EOF
}

while (($# > 0)); do
    case "$1" in
        --purge-data) purge_data=true ;;
        --yes) assume_yes=true ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
    shift
done

if ((EUID != 0)); then
    echo "Run this script as root: sudo bash uninstall.sh" >&2
    exit 1
fi

if command -v systemctl >/dev/null 2>&1; then
    systemctl disable --now "$service_name" >/dev/null 2>&1 || true
fi

rm -f -- "$service_file" "$binary_file"
rm -rf -- "$service_dropins"

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
    systemctl reset-failed "$service_name" >/dev/null 2>&1 || true
fi

if [[ "$purge_data" == true ]]; then
    if [[ "$assume_yes" != true ]]; then
        echo "This permanently deletes ${data_root}, including the database and master key."
        read -r -p 'Type DELETE to continue: ' confirmation
        if [[ "$confirmation" != "DELETE" ]]; then
            echo "Data deletion cancelled. Manager data remains in ${data_root}."
            exit 0
        fi
    fi
    [[ "$data_root" == "/var/lib/super-proxy-manager" ]]
    rm -rf -- "$data_root"
    echo "Super-Proxy Manager and its data were removed."
else
    echo "Super-Proxy Manager was removed. Data remains in ${data_root}."
fi
