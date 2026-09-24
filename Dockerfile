# Multi-stage build for makeutility-api. Base images are pinned to both a
# tag (for readability) and a digest (for reproducible, tamper-proof
# builds); see proposal.md's "Docker" section for why.

FROM golang:1.27@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/makeutility-api ./cmd/makeutility-api

FROM alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8
RUN apk add --no-cache ca-certificates
RUN adduser -D -H -s /sbin/nologin makeutility
USER makeutility

COPY --from=builder /out/makeutility-api /usr/local/bin/makeutility-api

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/makeutility-api"]
