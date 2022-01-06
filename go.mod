module github.com/reddit/baseplate.go

go 1.17

require (
	github.com/Shopify/sarama v1.29.1
	github.com/alicebob/miniredis/v2 v2.14.3
	github.com/apache/thrift v0.15.0
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/getsentry/sentry-go v0.11.0
	github.com/go-kit/kit v0.9.0
	github.com/go-redis/redis/v8 v8.10.0
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/google/go-cmp v0.5.6
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/joomcode/errorx v1.0.3
	github.com/joomcode/redispipe v0.9.4
	github.com/opentracing/opentracing-go v1.2.0
	github.com/sony/gobreaker v0.4.1
	go.uber.org/zap v1.15.0
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1
	google.golang.org/grpc v1.41.0
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/yaml.v2 v2.3.0
)

require (
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/alicebob/gopher-json v0.0.0-20200520072559-a9ecdc9d1d3a // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/chasex/redis-go-cluster v1.0.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/eapache/go-resiliency v1.2.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/garyburd/redigo v1.6.2 // indirect
	github.com/go-logfmt/logfmt v0.4.0 // indirect
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.2 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/klauspost/compress v1.12.2 // indirect
	github.com/kr/logfmt v0.0.0-20140226030751-b84e30acd515 // indirect
	github.com/mediocregopher/radix.v2 v0.0.0-20181115013041-b67df6e626f9 // indirect
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/yuin/gopher-lua v0.0.0-20200816102855-ee81675732da // indirect
	go.opentelemetry.io/otel v0.20.0 // indirect
	go.opentelemetry.io/otel/metric v0.20.0 // indirect
	go.opentelemetry.io/otel/trace v0.20.0 // indirect
	go.uber.org/atomic v1.6.0 // indirect
	go.uber.org/multierr v1.5.0 // indirect
	golang.org/x/crypto v0.0.0-20210616213533-5ff15b29337e // indirect
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/text v0.3.6 // indirect
	golang.org/x/tools v0.1.3-0.20210608163600-9ed039809d4c // indirect
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	honnef.co/go/tools v0.2.0 // indirect
)

replace github.com/reddit/baseplate.go/internal/limitopen => ./internal/limitopen
