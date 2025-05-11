package config

import (
	"flag"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env                 string        `yaml:"env" env-default:"local"`
	StoragePath         string        `yaml:"storage_path" env-required:"true"`
	TokenTTL            time.Duration `yaml:"token_ttl" env-required:"true"`
	TimeAdditon         time.Duration `yaml:"TIME_ADDITION_MS" env-required:"true"`
	TimeSubtraction     time.Duration `yaml:"TIME_SUBTRACTION_MS" env-required:"true"`
	TimeMultiplications time.Duration `yaml:"TIME_MULTIPLICATIONS_MS" env-required:"true"`
	TimeDivisions       time.Duration `yaml:"TIME_DIVISIONS_MS" env-required:"true"`
	ComputingPower      int           `yaml:"COMPUTING_POWER" env-required:"true"`
	Port                int           `yaml:"port" env-required:"true"`
	GRPCPort            int           `yaml:"grpc_port" env-required:"true"`
}

func MustLoad() *Config {
	path := fetchConfigPath()
	if path == "" {
		panic("config path is empty")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		panic("config file does not exist: " + path)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		panic("failed to read config: " + err.Error())
	}

	return &cfg
}

func fetchConfigPath() string {
	var res string

	// --config="config/local.yaml"
	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		return ""
	}

	return res
}
