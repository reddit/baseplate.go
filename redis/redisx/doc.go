// Package redisx wraps https://github.com/joomcode/redispipe in an interface where
// rather than returning the result, you pass in a pointer to the variable you want
// to put the result into  and it uses reflection to do that.
//
// It is inspired by https://github.com/jmoiron/sqlx and  https://github.com/mediocregopher/radix
package redisx
