#!/bin/bash

set -e

VERSION=${1:-"1.0.0"}
BUILD_DIR="build"
DIST_DIR="dist"

# Clean previous builds
rm -rf "$BUILD_DIR" "$DIST_DIR"
mkdir -p "$BUILD_DIR" "$DIST_DIR"

# Platforms to build for
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/arm64"
)

echo "Building MCP Memory Server v${VERSION}..."

for platform in "${PLATFORMS[@]}"; do
    OS=$(echo $platform | cut -d'/' -f1)
    ARCH=$(echo $platform | cut -d'/' -f2)
    
    output_name="mcp-memory-${OS}-${ARCH}"
    if [ "$OS" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    echo "Building for ${OS}/${ARCH}..."
    
    GOOS=$OS GOARCH=$ARCH go build -o "$BUILD_DIR/$output_name" main.go
    
    # Create archive
    if [ "$OS" = "windows" ]; then
        # Create zip for Windows
        cd "$BUILD_DIR"
        zip -q "../$DIST_DIR/${output_name}.zip" "$output_name"
        cd ..
        sha256sum "$DIST_DIR/${output_name}.zip" > "$DIST_DIR/${output_name}.zip.sha256"
    else
        # Create tarball for Unix
        tar -czf "$DIST_DIR/${output_name}.tar.gz" -C "$BUILD_DIR" "$output_name"
        sha256sum "$DIST_DIR/${output_name}.tar.gz" > "$DIST_DIR/${output_name}.tar.gz.sha256"
    fi
done

# Create install script tarball
tar -czf "$DIST_DIR/install.sh.tar.gz" install.sh
sha256sum "$DIST_DIR/install.sh.tar.gz" > "$DIST_DIR/install.sh.tar.gz.sha256"

echo ""
echo "Build complete! Files in $DIST_DIR:"
ls -lh "$DIST_DIR"
