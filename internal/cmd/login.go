// Package cmd for handling commands
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	bip39 "github.com/cosmos/go-bip39"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/grid3-go/internal/config"
)

// Login handles login command logic
func Login() error {
	scanner := bufio.NewReader(os.Stdin)

	fmt.Print("Please enter your mnemonics: ")
	mnemonics, err := scanner.ReadString('\n')
	if err != nil {
		return errors.Wrap(err, "failed to read mnemonics")
	}
	mnemonics = strings.TrimSpace(mnemonics)
	if !bip39.IsMnemonicValid(mnemonics) {
		return errors.New("failed to validate mnemonics")
	}

	fmt.Print("Please enter grid network (main,test): ")
	network, err := scanner.ReadString('\n')
	if err != nil {
		return errors.Wrap(err, "failed to read grid network")
	}
	network = strings.TrimSpace(network)

	if network != "dev" && network != "qa" && network != "test" && network != "main" {
		return errors.New("invalid grid network, must be one of: dev, test, qa and main")
	}
	path, err := config.GetConfigPath()
	if err != nil {
		return errors.Wrap(err, "failed to get configuration file")
	}

	var cfg config.Config
	cfg.Mnemonics = mnemonics
	cfg.Network = network

	err = cfg.Save(path)
	if err != nil {
		return err
	}
	log.Info().Msg("configuration saved")
	return nil
}
