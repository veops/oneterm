#!/bin/sh

KEY_PATH="$HOME/.ssh/id_ed25519"

if [ -f "$KEY_PATH" ]; then
    echo "SSH key already exists at $KEY_PATH. Skipping key generation."
else
    echo "SSH key does not exist. Generating new key."
    mkdir -p ~/.ssh
    ssh-keygen -t ed25519 -f "$KEY_PATH" -N ''
fi

./ssh ssh -c config.yaml
