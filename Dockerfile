FROM golang:1.19-alpine as builder

RUN apk add --quiet --no-cache build-base

RUN mkdir /build

ADD . /build/

WORKDIR /build

RUN go build -o lightningtipbot .

#building finished. Now extracting single bin in second stage.
FROM alpine

COPY --from=builder /build/lightningtipbot /app/

WORKDIR /app

CMD ["./lightningtipbot"]
