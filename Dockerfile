FROM golang:1.14 as builder
WORKDIR /alertmanager-discord
COPY go.mod go.sum /alertmanager-discord/
RUN go mod download
COPY . /alertmanager-discord
RUN CGO_ENABLED=0 go install .

FROM gcr.io/distroless/static-debian10
EXPOSE 9094
COPY --from=builder /go/bin/alertmanager-discord /go/bin/alertmanager-discord
ENTRYPOINT ["/go/bin/alertmanager-discord"]
