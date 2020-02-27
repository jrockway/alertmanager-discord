FROM golang:1.13-alpine as builder
# Install SSL ca certificates
RUN apk update && apk add git && apk add ca-certificates
# Create appuser
RUN adduser -D -g '' appuser
WORKDIR /alertmanager-discord
COPY go.mod go.sum /alertmanager-discord/
RUN go mod download
COPY . /alertmanager-discord
RUN CGO_ENABLED=0 go install .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/bin/alertmanager-discord /go/bin/alertmanager-discord

EXPOSE 9094
USER appuser
ENTRYPOINT ["/go/bin/alertmanager-discord"]
