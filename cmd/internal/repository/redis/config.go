package redis

import "time"

type Config struct {
	Addr         string        `yaml:"addr"`
	Password     string        `yaml:"password"`
	User         string        `yaml:"user"`
	DB           int           `yaml:"db"`
	MaxRetries   int           `yaml:"max_retries"`
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}
