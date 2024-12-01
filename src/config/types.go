package config

import "github.com/rs/zerolog"

type Environment string

const (
	Dev  = "dev"
	Beta = "beta"
	Live = "live"
)

type Cfg struct {
	Env           Environment
	WebserverAddr string
	LogLevel      zerolog.Level
	EsBuild       EsBuildConfig
}

type EsBuildConfig struct {
	Port uint16
}
