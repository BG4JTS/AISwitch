#!/bin/bash
set -e
BASE=$(pwd)
cd "$BASE"
rm -f dist/*.exe dist/*.txt dist/ais-* 2>/dev/null

echo "=== Building default binaries (6 platforms) ==="
for pair in "linux/amd64" "linux/arm64" "windows/amd64" "windows/arm64" "darwin/amd64" "darwin/arm64"; do
  GOOS="${pair%/*}"
  GOARCH="${pair#*/}"
  name="ais-${GOOS}-${GOARCH}"
  [ "$GOOS" = "windows" ] && name="${name}.exe"
  echo "  → $name"
  GOOS="$GOOS" GOARCH="$GOARCH" go build -ldflags="-s -w" -o "dist/$name" . 2>&1
done

echo ""
echo "=== Building full binaries (all modules, 6 platforms) ==="
for pair in "linux/amd64" "linux/arm64" "windows/amd64" "windows/arm64" "darwin/amd64" "darwin/arm64"; do
  GOOS="${pair%/*}"
  GOARCH="${pair#*/}"
  name="ais-full-${GOOS}-${GOARCH}"
  [ "$GOOS" = "windows" ] && name="${name}.exe"
  echo "  → $name"
  GOOS="$GOOS" GOARCH="$GOARCH" go build -tags "cost,keymgr,webui" -ldflags="-s -w" -o "dist/$name" . 2>&1
done

echo ""
echo "=== Generating checksums ==="
cd dist
sha256sum ais-* *-full-* > checksums.txt 2>/dev/null || sha256sum * > checksums.txt 2>/dev/null
echo "DONE"
ls -lh | grep -E "ais-|checksum" 
