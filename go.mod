module github.com/reddit/baseplate.go

go 1.13

require (
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/apache/thrift v0.13.1-0.20200118205551-397645ac2487
	github.com/certifi/gocertifi v0.0.0-20191021191039-0944d244cd40 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/getsentry/raven-go v0.2.0
	github.com/go-kit/kit v0.9.0
	github.com/go-logfmt/logfmt v0.4.0 // indirect
	github.com/go-redis/redis/v7 v7.0.0-beta.5
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/opentracing/opentracing-go v1.1.0
	go.uber.org/zap v1.13.0
	golang.org/x/sys v0.0.0-20191210023423-ac6580df4449
	gopkg.in/dgrijalva/jwt-go.v3 v3.2.0
	gopkg.in/fsnotify.v1 v1.4.7
)

replace gopkg.in/dgrijalva/jwt-go.v3 => github.com/reddit/jwt-go v3.2.1-0.20200222044038-a63f2d40479f+incompatible
