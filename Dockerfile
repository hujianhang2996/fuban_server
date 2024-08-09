FROM golang:1.22-bullseye as base

WORKDIR $GOPATH/src/smallest-golang/app/

COPY . .

RUN go mod download
# RUN go mod download
# RUN go mod verify
# RUN go get github.com/shirou/gopsutil/v4/mem

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /main .

FROM gcr.io/distroless/static-debian11

COPY --from=base /main .

CMD ["./main"]