FROM docker.io/library/golang:1.22 as builder
COPY . .

RUN make compile
FROM registry.access.redhat.com/ubi9/ubi-micro:latest

COPY --from=builder /go/build/ /releases
