module github.com/Snowflake-Labs/sansshell

go 1.19

require (
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/go-logr/logr v1.2.4
	github.com/go-logr/stdr v1.2.2
	github.com/google/go-cmp v0.5.9
	github.com/google/subcommands v1.2.0
	github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus v1.0.0-rc.0
	github.com/open-policy-agent/opa v0.50.2
	github.com/prometheus/client_golang v1.14.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.40.0
	go.opentelemetry.io/otel v1.14.0
	go.opentelemetry.io/otel/exporters/prometheus v0.37.0
	go.opentelemetry.io/otel/metric v0.37.0
	go.opentelemetry.io/otel/sdk/metric v0.37.0
	go.opentelemetry.io/otel/trace v1.14.0
	gocloud.dev v0.29.0
	golang.org/x/sync v0.1.0
	golang.org/x/sys v0.7.0
	google.golang.org/grpc v1.54.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.3.0
	google.golang.org/protobuf v1.30.0
	gopkg.in/ini.v1 v1.67.0
)

require (
	cloud.google.com/go v0.109.0 // indirect
	cloud.google.com/go/compute v1.18.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/iam v0.10.0 // indirect
	cloud.google.com/go/storage v1.29.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.3.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.2.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.1.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.0.0 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v0.8.1 // indirect
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/agnivade/levenshtein v1.1.1 // indirect
	github.com/aws/aws-sdk-go v1.44.200 // indirect
	github.com/aws/aws-sdk-go-v2 v1.17.4 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.4.10 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.18.12 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.13.12 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.22 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.11.51 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.28 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.22 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.29 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.0.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.9.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.1.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.13.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.30.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.14.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.18.3 // indirect
	github.com/aws/smithy-go v1.13.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.3 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/google/wire v0.5.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.3 // indirect
	github.com/googleapis/gax-go/v2 v2.7.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0-rc.3 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.39.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/tchap/go-patricia/v2 v2.3.1 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/yashtewari/glob-intersection v0.1.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel/sdk v1.14.0 // indirect
	golang.org/x/crypto v0.6.0 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/oauth2 v0.5.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/api v0.110.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230209215440-0dfe4f8abfcc // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
