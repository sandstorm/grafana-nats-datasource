#!/bin/bash
############################## DEV_SCRIPT_MARKER ##############################
# This script is used to document and run recurring tasks in development.     #
#                                                                             #
# You can run your tasks using the script `./dev some-task`.                  #
# You can install the Sandstorm Dev Script Runner and run your tasks from any #
# nested folder using `dev some-task`.                                        #
# https://github.com/sandstorm/Sandstorm.DevScriptRunner                      #
###############################################################################

source ./dev_utilities.sh

set -e

######### TASKS #########

# Easy setup of the project
function setup() {
  _log_green "Setting up your project"
  brew install mage
}

# Build the Go Backend of the NATS Grafana datasource
function build-backend() {
  go mod tidy
  mage -v
  cd ../
  _log_green "Built"
}

# Watch (and initially build) the Frontend of the NATS Grafana datasource
function watch-frontend() {
  yarn install
  yarn dev
  _log_green "Finished"
}

# Start a dev grafana instance
function up() {
  docker compose up
  _log_green "Finished"
}

function logs() {
  docker compose logs -f
  _log_green "Finished"
}

function grafana-reload-plugin() {
  grafana-build-backend
  cd sandstormmedia-nats-datasource
  # does not seem to work!?
  #docker compose exec grafana pkill gpx_nats_linux_amd64
  #docker compose exec grafana pkill gpx_nats_linux_arm64

  # this works :)
  docker compose restart
  _log_green "Reloaded plugin"
}

function test-nats-server() {
  # see https://spin.atomicobject.com/2017/08/24/start-stop-bash-background-process/ for cleanup logic
  trap "exit" INT TERM ERR
  trap "kill 0" EXIT

  # full logging
  nats --user=example --password=pass sub '>' &
  # nats --user=example --password=pass pub test --count 1000 "Message {{Count}}"  --sleep=1s


  go run e2etest/example_nats_message_publisher.go
  wait
}

function enter() {
  docker compose exec grafana /bin/bash
}

_log_green "---------------------------- RUNNING TASK: $1 ----------------------------"

# THIS NEEDS TO BE LAST!!!
# this will run your tasks
"$@"
