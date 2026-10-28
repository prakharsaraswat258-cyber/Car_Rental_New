# Image for Cloudflare Containers (see wrangler.jsonc). Pure-Go SQLite, so
# the binary is static and the runtime image only needs a writable /data.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/saarthi ./cmd/saarthi

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -H app && mkdir /data && chown app /data
COPY --from=build /out/saarthi /usr/local/bin/saarthi
USER app
ENV PORT=8080 SAARTHI_DATA=/data
EXPOSE 8080
CMD ["saarthi"]
