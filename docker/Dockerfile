FROM golang:1.19-alpine3.18 AS builder

WORKDIR /app

RUN apk update \
  && apk add --virtual build-deps gcc git curl tar \
  && rm -rf /var/cache/apk/*

COPY . .

RUN curl -L https://github.com/nyaruka/goflow/releases/download/v$(grep goflow go.mod | cut -d" " -f2 | cut -c2-)/docs.tar.gz --output docs.tar.gz && \
    tar -xf docs.tar.gz && rm docs.tar.gz

RUN --mount=type=cache,target=/go/pkg/mod/ \
  go install -v ./cmd/...

FROM alpine:3.18

COPY --from=builder /go/bin/ /app/
COPY --from=builder /app/docs /app/docs

RUN apk add --no-cache tzdata

WORKDIR /app

EXPOSE 8000
ENTRYPOINT ["./mailroom"]
