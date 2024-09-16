#!/bin/bash

# Define variables
GO_URL="https://go.dev/dl/"
LATEST_GO_VERSION=$(curl -sL https://go.dev/VERSION?m=text)
DOWNLOAD_URL="${GO_URL}${LATEST_GO_VERSION}.darwin-arm64.tar.gz"
INSTALL_DIR="/usr/local"
TEMP_DIR="/tmp/go-update"

# Create a temporary directory
mkdir -p $TEMP_DIR
cd $TEMP_DIR

# Download the latest Go version
echo "Downloading Go $LATEST_GO_VERSION..."
curl -LO $DOWNLOAD_URL

# Remove old Go version
if [ -d "$INSTALL_DIR/go" ]; then
  echo "Removing old Go version..."
  sudo rm -rf $INSTALL_DIR/go
fi

# Install the new version
echo "Installing Go $LATEST_GO_VERSION..."
sudo tar -C $INSTALL_DIR -xzf ${LATEST_GO_VERSION}.darwin-arm64.tar.gz

# Clean up
cd ~
rm -rf $TEMP_DIR

# Set Go environment variables
echo "Setting up Go environment variables..."
echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.zshrc
source ~/.zshrc

# Verify installation
echo "Go version installed:"
go version

echo "Go has been updated to $LATEST_GO_VERSION."