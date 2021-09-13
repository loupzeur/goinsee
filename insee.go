package utils

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	inseeAuthUrl       = "https://api.insee.fr/token"
	inseeCheckUrl      = "https://api.insee.fr/entreprises/sirene/V3/siren/"
	inseeTokenValidity = 604800
)

//Insee basic object to manage the API
//https://api.gouv.fr/documentation/sirene_v3
type Insee struct {
	AuthKey       string
	AuthSecret    string
	AuthToken     InseeToken
	Authed        bool
	AuthLastToken time.Time
}

//InseeToken to manage token auth response
type InseeToken struct {
	Token   string `json:"access_token"`
	Type    string `json:"token_type"`
	Scope   string `json:"scope"`
	Expires int    `json:"expires_in"`
}

//NewInsee create a non refreshed token Insee stuff
func NewInsee(authKey string, authSecret string) Insee {
	i := Insee{AuthKey: authKey, AuthSecret: authSecret}
	i.SetAuthToken()
	return i
}

//NewInseeRefreshed create a refreshed token Insee stuff
func NewInseeRefreshed(authKey string, authSecret string) Insee {
	i := NewInsee(authKey, authSecret)
	i.RefreshAuthToken()
	return i
}

//SetAuthToken will set Token from given Key and Secret
func (i *Insee) SetAuthToken() (err error) {
	//no need to refresh token before a while
	if i.Authed && i.AuthLastToken.Before(i.AuthLastToken.Add(time.Second*600000)) {
		return
	}
	if i.AuthKey == "" || i.AuthSecret == "" {
		return errors.New("invalid auth token or secret")
	}
	msg := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", i.AuthKey, i.AuthSecret)))

	i.AuthLastToken = time.Now()
	i.Authed = false

	//request to regen auth token
	data := url.Values{"grant_type": []string{"client_credentials"}}
	req, _ := http.NewRequest("POST", inseeAuthUrl, strings.NewReader(data.Encode()))
	req.Header.Add("Authorization", "Basic "+msg)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err == nil {
		s, _ := ioutil.ReadAll(resp.Body)
		ret := InseeToken{}
		json.Unmarshal(s, &ret)
		//string():"{\"access_token\":\"token\",\"scope\":\"am_application_scope default\",\"token_type\":\"Bearer\",\"expires_in\":603323}"
		i.AuthToken = ret
		inseeTokenValidity = ret.Expires
		i.Authed = true
	}
	return
}

//RefreshAuthToken automatically refresh the auth token based on expiry time
func (i *Insee) RefreshAuthToken() (err error) {
	err = i.SetAuthToken()
	if err != nil {
		return
	}
	go func() {
		//refreshing every 7 days approximately
		td := time.Duration(inseeTokenValidity - 60)
		time.Sleep(time.Second * td)
		i.RefreshAuthToken()
	}()
	return
}

//SirenExist return if the siren exist
func (i *Insee) SirenExist(siren string) bool {
	if !i.Authed || i.AuthToken.Token == "" {
		return false
	}
	req, _ := http.NewRequest("GET", inseeCheckUrl+siren, nil)
	req.Header.Add("Authorization", i.AuthToken.Type+" "+i.AuthToken.Token)
	client := &http.Client{}
	resp, err := client.Do(req)
	return err == nil && resp.StatusCode == 200
}
