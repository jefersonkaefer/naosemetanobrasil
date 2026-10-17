FROM node:24.19.0-bookworm-slim AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-fund --no-audit
COPY web/ ./
RUN npm run build

FROM golang:1.27.2-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/migrate ./cmd/migrate \
 && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/admin ./cmd/admin

FROM gcr.io/distroless/static-debian12:nonroot
# Render secret files are readable by group 1000; keep the process non-root.
USER 65532:1000
COPY --from=build /out/ /app/
COPY --from=frontend /src/web/dist/ /app/web/
ENV APP_ENV=production PORT=8080 FRONTEND_DIR=/app/web
EXPOSE 8080
ENTRYPOINT ["/app/api"]
