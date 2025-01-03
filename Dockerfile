FROM golang:1.23-alpine AS builder

RUN apk add --no-cache make

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN make build

FROM alpine:latest

WORKDIR /root/

RUN apk --no-cache add sqlite-libs

COPY --from=builder /app/bin/main .
COPY --from=builder /app/.env .

EXPOSE 8080

CMD ["./main"]
