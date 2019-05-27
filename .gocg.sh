#!/usr/bin/env bash

# 任何命令返回非0值退出
set -o errexit
# 使用未定义的变量退出
set -o nounset
# 管道中任一命令执行失败退出
set -o pipefail

BINARY_NAME=discovery
MAIN_FILE="./cmd/discovery/main.go"
INCLUDE_FILE=(configs)
