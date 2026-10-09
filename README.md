# The Bridge

An independent, Chelsea-inspired fan site concept with a responsive HTML/CSS/JavaScript frontend and a Go standard-library backend. This is a fan-made demo, not an official Chelsea FC website. Match centre content is sample data, not live scores.

## Run

Requires Go 1.22 or newer.

```sh
go run .
```

Open [http://localhost:8080](http://localhost:8080). The frontend is embedded into the Go binary at build time, so no Node.js installation or separate asset server is needed.

## API

`GET /api/match-centre` returns the featured match, upcoming fixtures, and editorial story data as JSON.

## Test

```sh
go test ./...
```# test-repo
nil
