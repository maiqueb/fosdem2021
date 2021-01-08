FROM golang:1.14 as builder
ENV GOPATH=/go
WORKDIR /go/src/github.com/maiqueb/fosdem2021-capabilities-demo
COPY . .
RUN GOOS=linux CGO_ENABLED=e go build -o /capabilities-demo \
        github.com/maiqueb/fosdem2021-capabilities-demo/cmd

FROM fedora:32
RUN dnf install -y iproute \
    procps-ng
COPY --from=builder /capabilities-demo /capabilities-demo
