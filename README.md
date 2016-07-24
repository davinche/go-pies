# Go-Pies

## Building

`go build`

This will output a binary called `gpies`.

## Deploying

Copy the `gpies` binary onto the server. Make sure `config.json` and `pies.json` are also in the same directory as the binary.

## Running

`./gpies`

Runs on port 31415.

eg: <http://localhost:31415>

### Optional Flags

1. `-i` Ingest flag: Specify this to flush and repopulate redis
2. `-s` Ingest Source: Specify a URL to ingest from.

Example:

`./gpies -i -s http://example.com/pies.json`

### Note

If the ingest flag (`-i`) is specified but no source is provided, it will use the `pies.json` (that we copied over from the deployment step) to repopulate redis.

