FROM golang:1.16.3-buster as build

ADD . /pleco
WORKDIR /pleco
RUN go get && go build -o /pleco.bin main.go

FROM debian:buster-slim as run

RUN apt-get update && apt-get install -y ca-certificates && apt-get clean
COPY --from=build /pleco.bin /usr/bin/pleco
CMD ["pleco", "start"]
