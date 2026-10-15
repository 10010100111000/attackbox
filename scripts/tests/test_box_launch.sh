#!/bin/bash
# scripts/test_box_launch.sh - 校验 box-launch.sh 语法
set -e
bash -n scripts/box-launch.sh
echo "PASS: box-launch.sh syntax valid"
