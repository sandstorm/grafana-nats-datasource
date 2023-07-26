#!/bin/bash

set -e

####### Utilities #######

_log_green() {
  printf "\033[0;32m%s\033[0m\n" "${1}"
}
_log_yellow() {
  printf "\033[1;33m%s\033[0m\n" "${1}"
}
_log_red() {
  printf "\033[0;31m%s\033[0m\n" "${1}"
}
