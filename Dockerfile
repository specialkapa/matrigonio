# ---- build stage ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache deps first for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO disabled -> a fully static binary that runs on a scratch/alpine base.
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/matrigonio .

# ---- run stage ----
FROM alpine:3.20
WORKDIR /app

# The app loads these by relative path at runtime, so keep the same layout.
COPY --from=build /out/matrigonio /app/matrigonio
COPY web/static       /app/web/static
COPY internal/templates /app/internal/templates
COPY internal/data    /app/internal/data

ENV PLATFORM=prod
EXPOSE 8081
ENTRYPOINT ["/app/matrigonio"]
