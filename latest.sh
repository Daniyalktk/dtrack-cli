#!/bin/bash
set -euo pipefail

OWNER="MedUnes"
REPO="dtrack-cli"
BINARY_PATTERN="dtrack-cli"
CHECKSUM_PATTERN="_checksums.txt"

if ! command -v curl &> /dev/null; then
    echo "Error: curl is not installed. Please install it."
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo "Error: jq is not installed. Please install it (e.g., sudo apt-get install jq)."
    exit 1
fi

if ! command -v sha256sum &> /dev/null; then
    echo "Error: sha256sum is not installed. Please install it (usually part of coreutils)."
    exit 1
fi

echo "Fetching latest release information for $OWNER/$REPO..."
LATEST_RELEASE_INFO=$(curl -s "https://api.github.com/repos/$OWNER/$REPO/releases/latest")

if [ -z "$LATEST_RELEASE_INFO" ]; then
  echo "Error: Failed to fetch latest release information from GitHub API."
  exit 1
fi

BINARY_URL=$(echo "$LATEST_RELEASE_INFO" | jq -r ".assets[] | select(.name | endswith(\"$BINARY_PATTERN\")) | .browser_download_url")

CHECKSUM_SUFFIX="_checksum.txt"
CHECKSUM_URL="$BINARY_URL$CHECKSUM_SUFFIX"

if [ -z "$BINARY_URL" ]; then
  echo "Error: Could not find the latest release asset matching the pattern '$BINARY_PATTERN'."
  exit 1
fi

if [ -z "$CHECKSUM_URL" ]; then
  echo "Error: Could not find the latest release asset matching the pattern '$CHECKSUM_PATTERN'."
  exit 1
fi

BINARY_FILE=$(basename "$BINARY_URL")
CHECKSUM_FILE=$(basename "$CHECKSUM_URL")

echo "Found latest package: $BINARY_FILE"
echo "Found latest checksum file: $CHECKSUM_FILE"

echo "Downloading package: $BINARY_FILE"
if ! wget -q -O "$BINARY_FILE" "$BINARY_URL"; then
    echo "Error: Failed to download package."
    exit 1
fi

echo "Downloading checksum file: $CHECKSUM_FILE"
if ! wget -q -O "$CHECKSUM_FILE" "$CHECKSUM_URL"; then
    echo "Error: Failed to download checksum file."
    rm -f "$BINARY_FILE"
    exit 1
fi
echo "Verifying checksum..."

EXPECTED_CHECKSUM=$(grep "$BINARY_FILE" "$CHECKSUM_FILE" | awk '{print $1}')

if [ -z "$EXPECTED_CHECKSUM" ]; then
    echo "Error: Could not find checksum for '$BINARY_FILE' in '$CHECKSUM_FILE'."
    rm -f "$BINARY_FILE" "$CHECKSUM_FILE"
    exit 1
fi

ACTUAL_CHECKSUM=$(sha256sum "$BINARY_FILE" | awk '{print $1}')

if [ "$ACTUAL_CHECKSUM" = "$EXPECTED_CHECKSUM" ]; then
    echo "Checksum verification successful."
else
    echo "Error: Checksum verification failed!"
    echo "  Expected: $EXPECTED_CHECKSUM"
    echo "  Actual:   $ACTUAL_CHECKSUM"
    rm -f "$BINARY_FILE" "$CHECKSUM_FILE"
    exit 1
fi


echo "Package downloaded successfully."
echo "Cleaning up downloaded files."
rm "$CHECKSUM_FILE"
exit 0