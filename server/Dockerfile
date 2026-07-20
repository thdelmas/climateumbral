FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /tilewhip-server .

FROM alpine:3.20
RUN adduser -D -H -u 1000 tilewhip
COPY --from=build /tilewhip-server /usr/local/bin/tilewhip-server
EXPOSE 8080
USER tilewhip
CMD ["tilewhip-server", "-addr", ":8080", "-data", "/data"]
