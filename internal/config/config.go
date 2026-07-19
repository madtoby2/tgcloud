package config

import (
	"encoding/json"
	"os"
	"strconv"
)

type Config struct {
	APIID      int    `json:"api_id"`
	APIHash    string `json:"api_hash"`
	ListenAddr string `json:"listen_addr"`
	DataDir    string `json:"data_dir"`
	AdminUser  string `json:"admin_user"`
	AdminPass  string `json:"admin_pass"`
}

func Default() *Config {
	return &Config{
		ListenAddr: ":8080",
		DataDir:    "./data",
		AdminUser:  "admin",
		AdminPass:  "tgcloud",
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Env overrides
	if v := os.Getenv("TGCLOUD_API_ID"); v != "" {
		cfg.APIID, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("TGCLOUD_API_HASH"); v != "" {
		cfg.APIHash = v
	}
	if v := os.Getenv("TGCLOUD_LISTEN"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("TGCLOUD_ADMIN_USER"); v != "" {
		cfg.AdminUser = v
	}
	if v := os.Getenv("TGCLOUD_ADMIN_PASS"); v != "" {
		cfg.AdminPass = v
	}

	return cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
