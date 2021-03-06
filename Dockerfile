FROM golang:1.15

RUN apt-get update && apt-get install -y curl jq

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["app"]
