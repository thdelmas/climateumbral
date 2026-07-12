# Production image: Go API serving the built frontend from one container.
# Mount the grid data at /data (see tools/fetch_grid.py or `make fetch`).
FROM node:22-alpine AS web
WORKDIR /app
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS api
WORKDIR /src
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /tilewhip-server .

FROM alpine:3.20
COPY --from=api /tilewhip-server /usr/local/bin/tilewhip-server
COPY --from=web /app/dist /srv/dist
VOLUME /data
EXPOSE 8080
CMD ["tilewhip-server", "-addr", ":8080", \
     "-data", "/data", "-dist", "/srv/dist"]
