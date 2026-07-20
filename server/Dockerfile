FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /climateumbral-server .

FROM alpine:3.20
RUN adduser -D -H -u 1000 climateumbral
COPY --from=build /climateumbral-server /usr/local/bin/climateumbral-server
EXPOSE 8080
USER climateumbral
CMD ["climateumbral-server", "-addr", ":8080", "-data", "/data"]
