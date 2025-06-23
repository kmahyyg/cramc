#!/bin/bash

if ! which yr >/dev/null 2>&1; then
  echo "[PRE-COMMIT] Failed to detect yara-x executable in your PATH.";
  exit 1;
fi

find assets/yrules/ -type f -iname '*.yar' -exec bash -c 'yr fmt "{}"' \;
exit 0