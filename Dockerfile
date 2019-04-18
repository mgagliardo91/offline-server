FROM golang:1.12
ENV GO111MODULE=on
WORKDIR $GOPATH/src/github.com/mgagliardo91/offline-server
COPY . .
RUN go get -d -v ./...
RUN go install -v ./...
EXPOSE 3000
CMD ["offline-server"]