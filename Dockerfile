# builder image
FROM golang:1.18.1 AS builder
WORKDIR /app
COPY . /app
RUN go mod download && go get -u -v -f all
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o servicesCommunication

FROM alpine:3.11.3
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/servicesCommunication .

ENTRYPOINT [ "/servicesCommunication" ]