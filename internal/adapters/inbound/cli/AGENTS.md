# internal/adapters/inbound/cli

## Purpose
The command-line tools' flags, usage and run loops, one sub-package per tool. Each `cmd/<tool>/main.go` is wiring only (`internal/arch/cmd_test.go`) and calls its sub-package's `Main`.

## Owns
`benchmark/`, `rambudget/`, `simulate/`: each exports `Main` returning the exit code, with its flags, output and logic beside it. Other tools add sibling sub-packages.

## Must not depend on
A sibling tool's package.
