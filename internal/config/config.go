// Package config for handling user configuration
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const configFile = ".tfgridconfig"

// Config struct that holds user configuration
type Config struct {
	Mnemonics string `json:"mnemonics"`
	Network   string `json:"network"`
}

// Save saves user configuration to tf-grid configuration file
func (c *Config) Save(path string) error {

	configFile, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "could not create configuration file %s", path)
	}
	defer configFile.Close()
	config := Config{
		Mnemonics: c.Mnemonics,
		Network:   c.Network,
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return errors.Wrapf(err, "could not marshal configuration data %+v", config)
	}
	_, err = configFile.Write(configJSON)
	if err != nil {
		return errors.Wrapf(err, "could not write configuration data to file %s", configFile.Name())
	}
	return nil
}

// Load loads user configuration from tf-grid configuration file
func (c *Config) Load(path string) error {
	configJSON, err := os.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "could not read configuration file %s", path)
	}
	err = json.Unmarshal(configJSON, &c)
	if err != nil {
		return errors.Wrapf(err, "could not unmarshal configuration data %s", configJSON)
	}
	return nil
}

// GetConfigPath returns the path of tf-grid configuration file
func GetConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", errors.Wrap(err, "could not get configuration directory")
	}
	path := filepath.Join(configDir, configFile)
	return path, nil
}

// GetUserConfig returns user configuration
func GetUserConfig() (Config, error) {

	path, err := GetConfigPath()
	if err != nil {
		return Config{}, errors.Wrap(err, "failed to get configuration file")
	}

	cfg := Config{}
	err = cfg.Load(path)
	if err != nil {
		return Config{}, errors.Wrap(err, "failed to load configuration try to login again using tf-grid login")
	}
	return cfg, nil
}
