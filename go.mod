module github.com/nyaruka/mailroom

go 1.23

require (
	firebase.google.com/go/v4 v4.14.1
	github.com/Masterminds/semver v1.5.0
	github.com/appleboy/go-fcm v1.2.1
	github.com/aws/aws-sdk-go-v2 v1.31.0
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.15.8
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.35.3
	github.com/aws/aws-sdk-go-v2/service/s3 v1.63.3
	github.com/buger/jsonparser v1.1.1
	github.com/elastic/go-elasticsearch/v8 v8.15.0
	github.com/getsentry/sentry-go v0.29.0
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-playground/validator/v10 v10.22.1
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/protobuf v1.5.4
	github.com/gomodule/redigo v1.9.2
	github.com/gorilla/schema v1.4.1
	github.com/jmoiron/sqlx v1.4.0
	github.com/lib/pq v1.10.9
	github.com/nyaruka/ezconf v0.3.0
	github.com/nyaruka/gocommon v1.59.1
	github.com/nyaruka/goflow v0.222.5
	github.com/nyaruka/null/v3 v3.0.0
	github.com/nyaruka/redisx v0.8.1
	github.com/nyaruka/rp-indexer/v9 v9.2.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/prometheus/client_model v0.6.1
	github.com/prometheus/common v0.60.0
	github.com/samber/slog-multi v1.2.2
	github.com/samber/slog-sentry v1.2.2
	github.com/shopspring/decimal v1.4.0
	github.com/stretchr/testify v1.9.0
	google.golang.org/api v0.199.0
)

require (
	cloud.google.com/go v0.115.1 // indirect
	cloud.google.com/go/auth v0.9.7 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.4 // indirect
	cloud.google.com/go/compute/metadata v0.5.2 // indirect
	cloud.google.com/go/firestore v1.17.0 // indirect
	cloud.google.com/go/iam v1.2.1 // indirect
	cloud.google.com/go/longrunning v0.6.1 // indirect
	cloud.google.com/go/storage v1.43.0 // indirect
	github.com/MicahParks/keyfunc v1.9.0 // indirect
	github.com/Shopify/gomail v0.0.0-20220729171026-0784ece65e69 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.5 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.39 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.37 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.14 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.18 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.18 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.23.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.3.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.9.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.17.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.23.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.27.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.31.3 // indirect
	github.com/aws/smithy-go v1.21.0 // indirect
	github.com/blevesearch/segment v0.9.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/elastic/elastic-transport-go/v8 v8.6.0 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/gabriel-vasile/mimetype v1.4.5 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/googleapis/gax-go/v2 v2.13.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/naoina/go-stringutil v0.1.0 // indirect
	github.com/naoina/toml v0.1.1 // indirect
	github.com/nyaruka/librato v1.1.1 // indirect
	github.com/nyaruka/null/v2 v2.0.3 // indirect
	github.com/nyaruka/phonenumbers v1.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/samber/lo v1.47.0 // indirect
	github.com/sergi/go-diff v1.3.1 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.55.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.55.0 // indirect
	go.opentelemetry.io/otel v1.30.0 // indirect
	go.opentelemetry.io/otel/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/trace v1.30.0 // indirect
	golang.org/x/crypto v0.27.0 // indirect
	golang.org/x/exp v0.0.0-20240909161429-701f63a606c0 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/oauth2 v0.23.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	golang.org/x/time v0.6.0 // indirect
	google.golang.org/appengine/v2 v2.0.6 // indirect
	google.golang.org/genproto v0.0.0-20240930140551-af27646dc61f // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240930140551-af27646dc61f // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240930140551-af27646dc61f // indirect
	google.golang.org/grpc v1.67.1 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
