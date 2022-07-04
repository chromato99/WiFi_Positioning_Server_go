# docker/dockerfile:1

FROM golang:1.18.3-alpine
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./


RUN go build -o  /WiFi_Positioning_Server_go

EXPOSE 8004

CMD [ "/WiFi_Positioning_Server_go" ]