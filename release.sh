#!/bin/bash

# Ensure a version argument is provided
if [ -z "$1" ]; then
  echo "Error: Please provide a version tag (e.g., v1.0.0)"
  echo "Usage: ./release.sh v1.0.0"
  exit 1
fi

VERSION=$1

# Make sure the version starts with 'v'
if [[ ! $VERSION == v* ]]; then
  echo "Error: Version must start with 'v' (e.g., v1.0.0)"
  exit 1
fi

echo "Creating tag $VERSION..."

# Create the git tag
git tag -a "$VERSION" -m "Release $VERSION"

echo "Pushing tag to origin..."

# Push the tag to GitHub (this triggers the GitHub Action / GoReleaser)
git push origin "$VERSION"

echo "Success! The Github Actions pipeline will now build the release."
