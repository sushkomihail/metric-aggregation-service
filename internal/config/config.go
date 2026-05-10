package config

import (
	"os"
	"strconv"
	"time"
)

const (
	defaultPostgresFlushInterval = 10 * time.Minute
	defaultPostgresStorageTime   = 12 * time.Hour

	defaultRedisDb           = 0
	defaultRedisMaxRetries   = 10
	defaultRedisDialTimeout  = 10 * time.Second
	defaultRedisReadTimeout  = 5 * time.Second
	defaultRedisWriteTimeout = 5 * time.Second
)

type Config struct {
	postgresConfig PostgresConfig
	redisConfig    RedisConfig
	kafkaConfig    KafkaConfig
	LogLevel       string
	MetricsPort    string
	GrpcPort       string
}

type PostgresConfig struct {
	Addr          string
	Port          string
	User          string
	Password      string
	DB            string
	FlushInterval time.Duration
	StorageTime   time.Duration
}

type RedisConfig struct {
	Addr         string
	Password     string
	User         string
	DB           int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type KafkaConfig struct {
	Servers string
}

func (c *Config) Load() {
	c.loadPostgresConfig()
	c.loadRedisConfig()
	c.loadKafkaConfig()

	c.LogLevel = os.Getenv("LOG_LEVEL")
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	c.MetricsPort = os.Getenv("METRICS_PORT")
	if c.MetricsPort == "" {
		c.MetricsPort = ":8081"
	}

	c.GrpcPort = os.Getenv("GRPC_PORT")
	if c.GrpcPort == "" {
		c.GrpcPort = ":8080"
	}
}

func (c *Config) PostgresConfig() PostgresConfig {
	return c.postgresConfig
}

func (c *Config) RedisConfig() RedisConfig {
	return c.redisConfig
}

func (c *Config) KafkaConfig() KafkaConfig {
	return c.kafkaConfig
}

func (c *Config) loadPostgresConfig() {
	c.postgresConfig.Addr = os.Getenv("POSTGRES_ADDR")
	c.postgresConfig.Port = os.Getenv("POSTGRES_PORT")
	c.postgresConfig.User = os.Getenv("POSTGRES_USER")
	c.postgresConfig.Password = os.Getenv("POSTGRES_PASSWORD")
	c.postgresConfig.DB = os.Getenv("POSTGRES_DB")

	flushInterval, err := time.ParseDuration(os.Getenv("POSTGRES_FLUSH_INTERVAL"))
	if err != nil {
		flushInterval = defaultPostgresFlushInterval
	}
	c.postgresConfig.FlushInterval = flushInterval

	storageTime, err := time.ParseDuration(os.Getenv("POSTGRES_STORAGE_TIME"))
	if err != nil {
		storageTime = defaultPostgresStorageTime
	}
	c.postgresConfig.StorageTime = storageTime
}

func (c *Config) loadRedisConfig() {
	c.redisConfig.Addr = os.Getenv("REDIS_ADDR")
	c.redisConfig.Password = os.Getenv("REDIS_PASSWORD")
	c.redisConfig.User = os.Getenv("REDIS_USER")

	db, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		db = defaultRedisDb
	}
	c.redisConfig.DB = db

	maxRetries, err := strconv.Atoi(os.Getenv("REDIS_MAX_RETRIES"))
	if err != nil {
		maxRetries = defaultRedisMaxRetries
	}
	c.redisConfig.MaxRetries = maxRetries

	dialTimeout, err := time.ParseDuration(os.Getenv("REDIS_DIAL_TIMEOUT"))
	if err != nil {
		dialTimeout = defaultRedisDialTimeout
	}
	c.redisConfig.DialTimeout = dialTimeout

	readTimeout, err := time.ParseDuration(os.Getenv("REDIS_READ_TIMEOUT"))
	if err != nil {
		readTimeout = defaultRedisReadTimeout
	}
	c.redisConfig.ReadTimeout = readTimeout

	writeTimeout, err := time.ParseDuration(os.Getenv("REDIS_WRITE_TIMEOUT"))
	if err != nil {
		writeTimeout = defaultRedisWriteTimeout
	}
	c.redisConfig.WriteTimeout = writeTimeout
}

func (c *Config) loadKafkaConfig() {
	c.kafkaConfig.Servers = os.Getenv("KAFKA_SERVERS")
}
