# 📬 Mailroom

[![Build Status](https://github.com/rapidpro/mailroom/workflows/CI/badge.svg)](https://github.com/rapidpro/mailroom/actions?query=workflow%3ACI)
[![codecov](https://codecov.io/gh/rapidpro/mailroom/branch/main/graph/badge.svg)](https://codecov.io/gh/rapidpro/mailroom)

Service for RapidPro/TextIt which does most of the heavy lifting. It interacts directly with the database and sends and 
receives messages with [Courier](https://github.com/nyaruka/courier) for handling via Redis.

## Deploying

As a Go application, it compiles to a binary and that binary along with the config file is all
you need to run it on your server. You can find bundles for each platform in the
[releases directory](https://github.com/rapidpro/mailroom/releases). We recommend running it
behind a reverse proxy such as nginx or Elastic Load Balancer that provides HTTPs encryption.

## Configuration

The service uses a tiered configuration system, each option takes precendence over the ones above it:

1.  The configuration file
2.  Environment variables starting with `MAILROOM_`
3.  Command line parameters

We recommend running it with no changes to the configuration and no parameters, using only
environment variables to configure it. You can use `% mailroom --help` to see a list of the
environment variables and parameters and for more details on each option.

For use with RapidPro/TextIt, you will need to configure these settings:

- `MAILROOM_ADDRESS`: address to bind our web server to (default "localhost")
- `MAILROOM_DOMAIN`: domain that mailroom is listening on
- `MAILROOM_AUTH_TOKEN`: authentication token clients will need to for web requests (should match setting in RapidPro)
- `MAILROOM_ATTACHMENT_DOMAIN`: domain that will be used for relative attachments in flows
- `MAILROOM_DB`: URL describing how to connect to the RapidPro database (default "postgres://temba:temba@localhost/temba?sslmode=disable")
- `MAILROOM_READONLY_DB`: URL for an additional database connection for read-only operations (optional)
- `MAILROOM_REDIS`: URL describing how to connect to Redis (default "redis://localhost:6379/15")
- `MAILROOM_SMTP_SERVER`: SMTP configuration for sending emails ex: smtp://user%40password@server:port/?from=foo%40gmail.com
- `MAILROOM_ANDROID_FCM_SERVICE_ACCOUNT_FILE`: FCM Service Account Credentials JSON File path used to notify Android relayers to sync
- `MAILROOM_ELASTIC_USERNAME`: ElasticSearch username for Basic Auth
- `MAILROOM_ELASTIC_PASSWORD`: ElasticSearch password for Basic Auth
- `MAILROOM_COURIER_AUTH_TOKEN`: authentication token used for requests to Courier

For writing of message attachments, you need an S3 compatible service which you configure with:

- `MAILROOM_AWS_ACCESS_KEY_ID`: AWS access key id used to authenticate to AWS
- `MAILROOM_AWS_SECRET_ACCESS_KEY` AWS secret access key used to authenticate to AWS
- `MAILROOM_S3_REGION`: region for your S3 bucket (ex: `eu-west-1`)
- `MAILROOM_S3_ATTACHMENTS_BUCKET`: name of your S3 bucket (ex: `mailroom-attachments`)
- `MAILROOM_S3_ATTACHMENTS_PREFIX`: prefix to use for filenames of attachments added to your bucket (ex: `attachments`)

You can use S3 storage for sessions and logs as well with:

- `MAILROOM_SESSION_STORAGE`: where session output is stored which must be `db` (default) or `s3`
- `MAILROOM_S3_SESSIONS_BUCKET`: name of your S3 bucket (ex: `mailroom-sessions`)
- `MAILROOM_S3_LOGS_BUCKET`: name of your S3 bucket (ex: `mailroom-logs`)

Flow engine configuration:

- `MAILROOM_MAX_STEPS_PER_SPRINT`: maximum number of steps allowed in a single engine sprint
- `MAILROOM_MAX_RESUMES_PER_SESSION`: maximum number of resumes allowed in an engine session
- `MAILROOM_MAX_VALUE_LENGTH`: maximum length in characters of contact field and run result values

Recommended settings for error and performance monitoring:

- `MAILROOM_LIBRATO_USERNAME`: username to use for logging of events to Librato
- `MAILROOM_LIBRATO_TOKEN`: token to use for logging of events to Librato
- `MAILROOM_SENTRY_DSN`: DSN to use when logging errors to Sentry
- `MAILROOM_LOG_LEVEL`: logging level mailroom should use (default is `warn`)

## Development

Once you've checked out the code, you can build the service with:

```
go build github.com/rapidpro/mailroom/cmd/mailroom
```

This will create a new executable in $GOPATH/bin called `mailroom`.

To run the tests you need to create the test database:

```
$ createdb mailroom_test
$ createuser -P -E -s mailroom_test (set no password)
```

To run all of the tests:

```
go test -p=1 ./...
```
