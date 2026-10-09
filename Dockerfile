# syntax=docker/dockerfile:1

# Build stage: compile a static binary with the web assets embedded.
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/thebridge .

# Runtime stage: minimal image, no shell, runs as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/thebridge /thebridge

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/thebridge"]
