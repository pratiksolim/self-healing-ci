FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/server ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=build /bin/server /app/server
WORKDIR /app

EXPOSE 8080
ENTRYPOINT ["/app/server"]
