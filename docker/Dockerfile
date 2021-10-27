FROM golang:1.16.8-bullseye AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download -x

COPY . ./

RUN curl -L https://github.com/nyaruka/goflow/releases/download/v$(grep goflow go.mod | cut -d" " -f2 | cut -c2-)/docs.tar.gz --output docs.tar.gz && \
    tar -xf docs.tar.gz && rm docs.tar.gz

RUN GOOS=linux GOARCH=amd64 go build -o /bin/mailroom ./cmd/mailroom/*.go

FROM gcr.io/distroless/base-debian11

WORKDIR /app

COPY --from=builder bin/mailroom ./

EXPOSE 8090
ENTRYPOINT ["./mailroom"]