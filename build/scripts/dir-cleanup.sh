#!/bin/bash

# Check if any arguments were provided
if [[ $# -eq 0 ]]; then
    echo "Usage: $0 <dir1> [dir2] [dir3] ..."
    exit 1
fi

# Store directories in an array
dirs=("$@")

# Display what will be removed
echo "The following directories will be removed:"
for dir in "${dirs[@]}"; do
    echo "  - $dir"
done
echo

# Ask for confirmation
read -p "Are you sure you want to remove these directories? [y/N] " response

if [[ "$response" =~ ^[Yy]$ ]]; then
    for dir in "${dirs[@]}"; do
        if [[ -d "$dir" || -f "$dir" ]]; then # optionally supports files if they were passed in
            sudo rm -rf "$dir"
            echo "Removed: $dir"
        else
            echo "(Skipped) directory already removed: $dir"
        fi
    done
    echo "Cleanup complete."
else
    echo "Cleanup cancelled."
    exit 0
fi
