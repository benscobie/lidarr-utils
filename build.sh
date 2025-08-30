#!/bin/bash

# Build script for multiple platforms

set -e

APP_NAME="lidarr-deduper"
VERSION=${1:-"dev"}
BUILD_DIR="build"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Building $APP_NAME version $VERSION${NC}"

# Clean build directory
rm -rf $BUILD_DIR
mkdir -p $BUILD_DIR

# Build targets
declare -a platforms=(
    "linux/amd64"
    "linux/arm64"
    "windows/amd64"
    "darwin/amd64"
    "darwin/arm64"
)

for platform in "${platforms[@]}"
do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}
    
    output_name="$BUILD_DIR/${APP_NAME}-${GOOS}-${GOARCH}"
    if [ $GOOS = "windows" ]; then
        output_name+='.exe'
    fi

    echo -e "${YELLOW}Building for $GOOS/$GOARCH...${NC}"
    
    env GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build \
        -ldflags="-w -s -X main.version=$VERSION" \
        -o $output_name \
        .
        
    if [ $? -ne 0 ]; then
        echo -e "${RED}Failed to build for $GOOS/$GOARCH${NC}"
        exit 1
    fi
done

# Create archives
echo -e "${YELLOW}Creating archives...${NC}"

cd $BUILD_DIR

for file in *; do
    if [[ $file == *".exe" ]]; then
        zip "${file%.exe}.zip" "$file" ../README.md ../LICENSE ../config.example.yaml
    else
        tar -czf "${file}.tar.gz" "$file" ../README.md ../LICENSE ../config.example.yaml
    fi
done

cd ..

echo -e "${GREEN}Build complete! Artifacts in $BUILD_DIR/${NC}"
ls -la $BUILD_DIR/
