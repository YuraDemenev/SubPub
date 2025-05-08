package config

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Config struct {
	GRPC struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"grpc"`
	Logging struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"logging"`
	SubPub struct {
		MessageChannelSize int `mapstructure:"message_channel_size"`
	} `mapstructure:"subpub"`
}

func LoadConfig() (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("../../config")

	//Read config
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("Failed to read config file err:%s", err.Error())
	}

	//create config struct
	var cfg Config

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %s", err.Error())
	}

	//Check validation
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %s", err.Error())
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.GRPC.Port <= 0 || c.GRPC.Port > 65535 {
		return fmt.Errorf("invalid grpc port: %d", c.GRPC.Port)
	}
	if c.SubPub.MessageChannelSize <= 0 {
		return fmt.Errorf("invalid message channel size %d", c.SubPub.MessageChannelSize)
	}
	//Check log level
	if _, err := logrus.ParseLevel(c.Logging.Level); err != nil {
		return fmt.Errorf("invalid logging level: %s", c.Logging.Level)
	}
	return nil
}
