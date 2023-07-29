package plugin

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"sync"
)

func (ds *Datasource) connectNats(options *MyDataSourceOptions, secureOptions *MySecureJsonData) (*nats.Conn, error) {
	ds.natsConnOnce.Do(func() {
		if options.Authentication == AuthenticationNone {
			ds.natsConn, ds.natsConnErr = nats.Connect(options.NatsUrl)
		} else if options.Authentication == AuthenticationNkey {
			ds.natsConn, ds.natsConnErr = nats.Connect(options.NatsUrl, nats.Nkey(
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
			ds.natsConn, ds.natsConnErr = nats.Connect(options.NatsUrl, nats.UserInfo(options.Username, secureOptions.Password))
		} else if options.Authentication == AuthenticationJWT {
			// Implemented after nats.UserCredentials(), but without temp file
			jwtAsByte := []byte(secureOptions.Jwt)
			userCB := func() (string, error) {
				return nkeys.ParseDecoratedJWT(jwtAsByte)
			}
			sigCB := func(nonce []byte) ([]byte, error) {
				keyPair, err := nkeys.ParseDecoratedNKey(jwtAsByte)
				if err != nil {
					return nil, fmt.Errorf("unable to extract key pair from file: %w", err)
				}
				// Wipe our key on exit.
				defer keyPair.Wipe()

				sig, _ := keyPair.Sign(nonce)
				return sig, nil
			}

			ds.natsConn, ds.natsConnErr = nats.Connect(options.NatsUrl, nats.UserJWT(userCB, sigCB))
		} else {
			// TODO: TOKEN AUTH
			ds.natsConnErr = fmt.Errorf("TODO")
		}
	})

	if ds.natsConnErr != nil {
		// in case of error, allow to re-run the initialization code above.
		// see https://github.com/golang/go/issues/25955#issuecomment-398278056
		// TODO: RACE CONDITION maybe?? (did not too closely think about it)
		ds.natsConnOnce = new(sync.Once)
	}

	return ds.natsConn, ds.natsConnErr
}
