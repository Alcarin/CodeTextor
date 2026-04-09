#!/bin/bash

# TODO: implement more features
source ./config.sh
. ./utils.sh

# A simple function
say_hello() {
  local name=$1
  echo "Hello, $name"
}

# Variable assignment
VERSION="1.0.0"
export PATH_TO_DB="/var/db"
readonly MAX_RETRIES=5

# Another function style
function get_version {
  echo $VERSION
}

# Alias
alias ll='ls -alF'

# Subshell
(
  echo "Inside subshell"
)
