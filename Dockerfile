FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o skills-browser .

FROM scratch
COPY --from=builder /src/skills-browser /skills-browser
ENTRYPOINT ["/skills-browser"]
