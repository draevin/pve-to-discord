FROM golang:bookworm

EXPOSE 80

WORKDIR /app

COPY . /app

RUN mkdir logs/

RUN go mod download

RUN go build -o /pvetodiscord

CMD [ "/pvetodiscord" ]