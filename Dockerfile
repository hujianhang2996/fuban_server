FROM golang:1.22-bullseye as base
COPY . .
RUN go mod download && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /main .
# FROM linuxcontainers/debian-slim:11
FROM gcr.io/distroless/static-debian11
COPY --from=base /main .
COPY --from=base /bin/df .
# RUN apt update && apt install hddtemp lm-sensors -y
CMD ["./main"]

# COPY . .
# RUN go mod download && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /main .
# FROM gcr.io/distroless/static-debian11
# COPY --from=base /main .
# CMD ["./main"]