FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o skill_browser ./cmd/skill_browser

FROM scratch
COPY --from=builder /src/skill_browser /skill_browser
ENTRYPOINT ["/skill_browser"]
