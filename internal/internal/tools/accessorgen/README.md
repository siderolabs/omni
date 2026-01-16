# go-accessor-gen

This tool creates getter and setter methods for Go structs.

It only works on fields that are pointers (like `*string` or `*int`).

## How to use

Run it on your Go file:

```bash
go run main.go --source input.go --output input.gen.go
```

This makes a new file named `input.gen.go`.

## Options

- `--source`: The file to read (required).
- `--output`: The file to write (required).
