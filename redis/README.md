# Redis packages in Baseplate.go

We support two different sets of Redis packages in Baseplate.go.

## Use Redis as a cache

If you use Redis as a cache, it's recommended to use
[`redispipebp`][redispipebp] + [`redisx`][redisx] packages.
They are built on top of [`redispipe`][redispipe] open source project.

This combination can also be used if you use Redis as a DB but only do readings
and not writings.

[redispipebp]: https://pkg.go.dev/github.com/reddit/baseplate.go/redis/cache/redispipebp
[redisx]: https://pkg.go.dev/github.com/reddit/baseplate.go/redis/cache/redisx
[redispipe]: https://github.com/joomcode/redispipe

## Use Redis as a DB

If you use Redis as a DB, and do writings,
it's recommended to use [`redisbp`][redisbp] package.
It's built on top of [`go-redis`][go-redis] open source project,
and provides [`Wait`][wait] function to help you make sure to reach the desired
consistency level on write operations.

There are also certain Redis features not supported by `redispipebp`,
like pubsub.
If you need to use those features then you should choose `redisbp` as well.

[redisbp]: https://pkg.go.dev/github.com/reddit/baseplate.go/redis/db/redisbp
[wait]: https://pkg.go.dev/github.com/reddit/baseplate.go/redis/db/redisbp#ClusterClient.Wait
[go-redis]: https://pkg.go.dev/github.com/go-redis/redis/v8
