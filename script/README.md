# Script Guide

This directory contains two separate Git scripts:

- `sync_upstream_to_zhiguofan.sh`: fetch upstream `main`, merge it into local `main`, then merge `main` into `zhiguofan` and push both branches to `origin`.
- `push_zhiguofan_to_internal_git.sh`: push the local `zhiguofan` branch to the internal Git server at `ssh://git_prod_backend@192.168.1.10/home/git_prod_backend/wmtoken_platform.git`.

The scripts are intentionally separate so you can run the GitHub/upstream sync in one network and the internal push in another.
