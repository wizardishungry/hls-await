# szyntax=docker/dockerfile:1
FROM golang:1.18rc1-bullseye

WORKDIR /usr/src/app

RUN apt update -y
RUN apt install -y libavfilter7 libavdevice58 libavformat-dev libavcodec-dev libavdevice-dev libavutil-dev libswscale-dev libswresample-dev libavfilter-dev

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/ ./...

CMD ["/usr/local/bin/watch"]