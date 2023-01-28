package plugin

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"os"
)

func (ds *Datasource) connectNats(options *MyDataSourceOptions, secureOptions *MySecureJsonData) (*nats.Conn, error) {
	var natsConn *nats.Conn
	var err error
	if options.Authentication == AuthenticationNone {
		natsConn, err = nats.Connect(options.NatsUrl)
	} else if options.Authentication == AuthenticationNkey {
		natsConn, err = nats.Connect(options.NatsUrl, nats.Nkey(
			options.Nkey,
			func(nonce []byte) ([]byte, error) {
				kp, err := nkeys.FromSeed([]byte(secureOptions.NkeySeed))
				if err != nil {
					return nil, fmt.Errorf("unable to load key pair from NkeySeed: %w", err)
				}
				// Wipe our key on exit.
				defer kp.Wipe()

				sig, _ := kp.Sign(nonce)
				return sig, nil
			},
		))
	} else if options.Authentication == AuthenticationUserPass {
		natsConn, err = nats.Connect(options.NatsUrl, nats.UserInfo(options.Username, secureOptions.Password))
	} else if options.Authentication == AuthenticationJWT {
		// WORKAROUND: store credentials in a temp-file
		file, err := os.CreateTemp("", "tmp-jwt")
		if err != nil {
			return nil, fmt.Errorf("TODO: %w", err)
		}
		defer os.Remove(file.Name())
		_, err = file.Write([]byte(secureOptions.Jwt))
		if err != nil {
			return nil, fmt.Errorf("TODO: %w", err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("TODO: %w", err)
		}

		natsConn, err = nats.Connect(options.NatsUrl, nats.UserCredentials(file.Name()))
	} else {
		// TODO: TOKEN AUTH
		return nil, fmt.Errorf("TODO")
	}

	return natsConn, err
}
