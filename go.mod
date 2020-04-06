module github.com/reddit/baseplate.go

go 1.14

require (
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/apache/thrift v0.13.1-0.20200417185339-9e864d57026b
	github.com/getsentry/sentry-go v0.6.0
	github.com/go-kit/kit v0.9.0
	github.com/go-logfmt/logfmt v0.4.0 // indirect
	github.com/go-redis/redis/v7 v7.0.0-beta.5
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/opentracing/opentracing-go v1.1.0
	go.uber.org/zap v1.13.0
	golang.org/x/sys v0.0.0-20191210023423-ac6580df4449
	gopkg.in/dgrijalva/jwt-go.v3 v3.2.0
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/yaml.v2 v2.2.4
)

replace gopkg.in/dgrijalva/jwt-go.v3 => github.com/reddit/jwt-go v3.2.1-0.20200222044038-a63f2d40479f+incompatible
