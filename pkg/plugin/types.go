package plugin

import "time"

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
	NkeySeed string `json:"nkeySeed"`
	Password string `json:"password"`
	Jwt      string `json:"jwt"`
}

const QueryTypeRequestReply = "REQUEST_REPLY"

type queryModel struct {
	QueryType      string        `json:"queryType"`
	NatsSubject    string        `json:"natsSubject"`
	RequestTimeout time.Duration `json:"requestTimeout"`
	RequestData    string        `json:"requestData"`
	JqExpression   string        `json:"jqExpression"`
}
