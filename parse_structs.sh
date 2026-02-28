#!/bin/bash
find service db pkg -name "*.go" -exec grep -Hn -A 5 "type.*struct" {} \;
