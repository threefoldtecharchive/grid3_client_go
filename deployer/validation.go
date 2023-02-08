// Package deployer for grid deployer
package deployer

import (
	"fmt"
	"log"
	"math/big"
	"net"
	"net/url"
	"regexp"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/substrate-client"
)

// validateAccount checks the mnemonics is associated with an account with key type ed25519
func validateAccount(tfPluginClient *TFPluginClient) error {
	sub := tfPluginClient.SubstrateConn
	_, err := sub.GetAccount(tfPluginClient.Identity)
	if err != nil && !errors.Is(err, substrate.ErrAccountNotFound) {
		return errors.Wrap(err, "failed to get account with the given mnemonics")
	}
	if err != nil { // Account not found
		funcs := map[string]func(string) (substrate.Identity, error){"ed25519": substrate.NewIdentityFromEd25519Phrase, "sr25519": substrate.NewIdentityFromSr25519Phrase}
		for keyType, f := range funcs {
			ident, err2 := f(tfPluginClient.Mnemonics)
			if err2 != nil { // shouldn't happen, return original error
				// TODO: set log level trace
				log.Printf("couldn't convert the mnemonics to %s key: %s", keyType, err2.Error())
				return err
			}
			_, err2 = sub.GetAccount(ident)
			if err2 == nil { // found an identity with key type other than the provided
				return fmt.Errorf("found an account with %s key type and the same mnemonics, make sure you provided the correct key type", keyType)
			}
		}
		// didn't find an account with any key type
		return err
	}
	return nil
}

// TODO: Remove validate Redis
// func validateRedis(tfPluginClient *TFPluginClient) error {
// 	errMsg := fmt.Sprintf("redis error. make sure rmb_redis_url is correct and there's a redis server listening there. rmb_redis_url: %s", tfPluginClient.RMBRedisURL)
// 	cl, err := newRedisPool(tfPluginClient.RMBRedisURL)
// 	if err != nil {
// 		return errors.Wrap(err, errMsg)
// 	}
// 	defer cl.Close()
// 	c, err := cl.Dial()
// 	if err != nil {
// 		return errors.Wrap(err, errMsg)
// 	}
// 	c.Close()
// 	return nil
// }

func validateYggdrasil(tfPluginClient *TFPluginClient, sub subi.SubstrateExt) error {
	yggIP, err := sub.GetTwinIP(tfPluginClient.TwinID)
	if err != nil {
		return errors.Wrapf(err, "could not get twin %d from substrate", tfPluginClient.TwinID)
	}
	ip := net.ParseIP(yggIP)
	listenIP := yggIP
	if ip != nil && ip.To4() == nil {
		// if it's ipv6 surround it with brackets
		// otherwise, keep as is (can be ipv4 or a domain (probably will fail later but we don't care))
		listenIP = fmt.Sprintf("[%s]", listenIP)
	}
	s, err := net.Listen("tcp", fmt.Sprintf("%s:0", listenIP))
	if err != nil {
		return errors.Wrapf(err, "couldn't listen on port. make sure the twin id is associated with a valid yggdrasil ip, twin id: %d, ygg ip: %s, err", tfPluginClient.TwinID, yggIP)
	}
	defer s.Close()
	port := s.Addr().(*net.TCPAddr).Port
	arrived := false
	go func() {
		c, err := s.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			return
		}
		arrived = true
		c.Close()
	}()
	c, err := net.Dial("tcp", fmt.Sprintf("%s:%d", listenIP, port))
	if err != nil {
		return errors.Wrapf(err, "failed to connect to ip. make sure the twin id is associated with a valid yggdrasil ip, twin id: %d, ygg ip: %s, err", tfPluginClient.TwinID, yggIP)
	}
	c.Close()
	if !arrived {
		return errors.Wrapf(err, "sent request but didn't arrive to me. make sure the twin id is associated with a valid yggdrasil ip, twin id: %d, ygg ip: %s, err", tfPluginClient.TwinID, yggIP)
	}
	return nil
}

// func validateRMB(tfPluginClient *TFPluginClient, sub subi.SubstrateExt) error {
// 	if err := validateRedis(tfPluginClient); err != nil {
// 		return err
// 	}

// 	return validateYggdrasil(tfPluginClient, sub)
// }

func validateRMBProxyServer(tfPluginClient *TFPluginClient) error {
	return tfPluginClient.GridProxyClient.Ping()
}

func validateMnemonics(mnemonics string) error {
	if len(mnemonics) == 0 {
		return errors.New("mnemonics required")
	}

	alphaOnly := regexp.MustCompile(`^[a-zA-Z\s]+$`)
	if !alphaOnly.MatchString(mnemonics) {
		return errors.New("mnemonics can only be composed of a non-alphanumeric character or a whitespace")
	}

	return nil
}

func validateSubstrateURL(url string) error {
	if len(url) == 0 {
		return errors.New("substrate url is required")
	}

	alphaOnly := regexp.MustCompile(`^wss:\/\/[a-z0-9]+\.[a-z0-9]\/?([^\s<>\#%"\,\{\}\\|\\\^\[\]]+)?$`)
	if !alphaOnly.MatchString(url) {
		return errors.New("substrate url is not valid")
	}

	return nil
}

func validateProxyURL(url string) error {
	if len(url) == 0 {
		return errors.New("proxy url is required")
	}

	alphaOnly := regexp.MustCompile(`^https:\/\/[a-z0-9]+\.[a-z0-9]\/?([^\s<>\#%"\,\{\}\\|\\\^\[\]]+)?$`)
	if !alphaOnly.MatchString(url) {
		return errors.New("proxy url is not valid")
	}

	return nil
}

// func validateClientRMB(tfPluginClient *TFPluginClient, sub subi.SubstrateExt) error {
// 	if tfPluginClient.UseRmbProxy {
// 		return validateRMBProxyServer(tfPluginClient)
// 	}

// 	return validateRMB(tfPluginClient, sub)
// }

func validateAccountBalanceForExtrinsics(sub subi.SubstrateExt, identity substrate.Identity) error {
	balance, err := sub.GetBalance(identity)
	if err != nil && !errors.Is(err, substrate.ErrAccountNotFound) {
		return errors.Wrap(err, "failed to get account with the given mnemonics")
	}
	log.Printf("money %d\n", balance.Free)
	if balance.Free.Cmp(big.NewInt(20000)) == -1 {
		return fmt.Errorf("account contains %s, min fee is 20000", balance.Free)
	}
	return nil
}

func newRedisPool(address string) (*redis.Pool, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	var host string
	switch u.Scheme {
	case "tcp":
		host = u.Host
	case "unix":
		host = u.Path
	default:
		return nil, fmt.Errorf("unknown scheme '%s' expecting tcp or unix", u.Scheme)
	}
	var opts []redis.DialOption

	if u.User != nil {
		opts = append(
			opts,
			redis.DialPassword(u.User.Username()),
		)
	}

	return &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial(u.Scheme, host, opts...)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) > 10*time.Second {
				//only check connection if more than 10 second of inactivity
				_, err := c.Do("PING")
				return err
			}

			return nil
		},
		MaxActive:   5,
		MaxIdle:     3,
		IdleTimeout: 1 * time.Minute,
		Wait:        true,
	}, nil
}
