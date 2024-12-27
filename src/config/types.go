package config

import "github.com/rs/zerolog"

type Environment string

const (
	Dev  = "dev"
	Beta = "beta"
	Live = "live"
)

/*
 * Most web frameworks allow you to define config files in a text format like
 * JSON or YAML. However, this requires you to come up with some fussy mapping
 * between text and arbitrary program structures, and therefore often to make
 * weird "factories" and deserialization methods. This is dumb and we prefer to
 * just write our config in the actual programming language and build it into
 * the binary at compile time. This works fine because we never distribute the
 * compiled binary on its own; in fact, we typically compile it directly on the
 * destination server.
 */

type Cfg struct {
	Env           Environment
	WebserverAddr string
	LogLevel      zerolog.Level
	EsBuild       EsBuildConfig
}

type EsBuildConfig struct {
	Port uint16
}
