package plugin

import (
	"encoding/json"
	"fmt"
	"time"
)

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
const QueryTypeSubscribe = "SUBSCRIBE"
const QueryTypeScript = "SCRIPT"

type queryModel struct {
	QueryType      string   `json:"queryType"`
	NatsSubject    string   `json:"natsSubject"`
	RequestTimeout Duration `json:"requestTimeout"`
	RequestData    string   `json:"requestData"`
	JsFn           string   `json:"jsFn"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var unmarshalledJson interface{}

	err := json.Unmarshal(b, &unmarshalledJson)
	if err != nil {
		return err
	}

	switch value := unmarshalledJson.(type) {
	case float64:
		d.Duration = time.Duration(value)
	case string:
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid duration: %#v", unmarshalledJson)
	}

	return nil
}
func (d Duration) MarshalJSON() (b []byte, err error) {
	return []byte(fmt.Sprintf(`"%s"`, d.String())), nil
}
