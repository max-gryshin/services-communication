FROM golang:1.23.2 AS builder
ARG OUTPUT_BINARY
ARG APP_VERSION
ARG BUILD_DIR
ADD ./app/
WORKDIR /app
RUN make build
#COPY . /app
#RUN go mod download && go get -u -v -f all
#RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o servicesCommunication

FROM alpine:3.13.5
ARG OUTPUT_BINARY
USER 0
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/$OUTPUT_BINARY /app/bin
RUN chown -R -f 888:888 /app \
 && chmod +x /app
USER 888
CMD [ "/app/bin" ]