#!/bin/bash

# 1. Get the absolute path of the directory where install.sh is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 2. Get the parent directory (where the 'scripts' folder lives)
PARENT_DIR="$(dirname "$SCRIPT_DIR")"

echo "Script is running from: $SCRIPT_DIR"
echo "Project root folder is: $PARENT_DIR"
