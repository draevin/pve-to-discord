FROM golang:alpine3.21 AS builder

WORKDIR /app

COPY . /app

RUN go mod download

RUN go build -o pvetodiscord

FROM alpine:latest

EXPOSE 80

WORKDIR /app

COPY --from=builder /app/pvetodiscord .

CMD [ "./pvetodiscord" ]