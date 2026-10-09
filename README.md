# The Bridge

An independent, Chelsea-inspired fan site concept with a responsive HTML/CSS/JavaScript frontend and a Go standard-library backend. This is a fan-made demo, not an official Chelsea FC website. Match centre content is sample data, not live scores.

## Run

Requires Go 1.26 or newer.

```sh
go run .
```

Open [http://localhost:8080](http://localhost:8080). The frontend is embedded into the Go binary at build time, so no Node.js installation or separate asset server is needed.

With no configuration the server keeps accounts in memory (lost on restart) and image uploads are disabled. Configure it with environment variables:

| Variable | Purpose |
| --- | --- |
| `MYSQL_DSN` | MySQL connection, e.g. `user:pass@tcp(host:3306)/thebridge`. Tables are created on startup. |
| `S3_BUCKET` | Bucket for uploaded profile and gallery photos. Enables uploads. |
| `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | Standard AWS settings. An attached IAM role or `~/.aws` profile also works. The app needs `s3:PutObject` and `s3:GetObject` on the bucket. |
| `S3_PUBLIC_URL` | Optional. Public base URL for the bucket (e.g. CloudFront). Without it the bucket can stay private and images are served through short-lived presigned links. |
| `S3_ENDPOINT` | Optional. An S3-compatible endpoint for local testing. |

### Docker Compose

Runs the app with MySQL. Export `S3_BUCKET`, `AWS_REGION` and AWS credentials first to enable uploads.

```sh
docker compose up --build
```

## API

| Endpoint | |
| --- | --- |
| `GET /api/match-centre` | Featured match, upcoming fixtures and editorial stories (sample data). |
| `POST /api/signup` | JSON `{name, email, password}`. Creates an account and signs in. |
| `POST /api/login` | JSON `{email, password}`. Sets an HTTP-only session cookie. |
| `POST /api/logout` | Ends the session. |
| `GET /api/session` | The signed-in user, if any. |
| `POST /api/account/avatar` | Multipart `image`. Sets the profile photo. |
| `GET /api/gallery` | Latest fan photos. |
| `POST /api/gallery` | Multipart `image` and optional `caption`. Signed-in users only. |
| `GET /media/{key}` | Redirects to an uploaded image in S3. |

Uploads must be JPEG, PNG, GIF or WebP (checked from the file contents) and at most 5 MB.

## Test

```sh
go test ./...
```

To also test the MySQL store, point `TEST_MYSQL_DSN` at an empty database:

```sh
TEST_MYSQL_DSN="root:secret@tcp(127.0.0.1:3306)/thebridge_test" go test ./...
```# test-repo
nil
