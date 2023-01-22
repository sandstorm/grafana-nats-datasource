package plugin

const AuthenticationNone = "NONE"
const AuthenticationNkey = "NKEY"
const AuthenticationUserPass = "USERPASS"
const AuthenticationJWT = "JWT"

type MyDataSourceOptions struct {
	NatsUrl        string `json:"natsUrl"`
	Authentication string `json:"authentication"`
	Nkey           string `json:"nkey"`
	Username       string `json:"username"`
}

type MySecureJsonData struct {
	NkeySeed []byte `json:"nkeySeed"`
	Password string `json:"password"`
	Jwt      []byte `json:"jwt"`
}
