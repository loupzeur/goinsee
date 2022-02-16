package goinsee

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/guregu/null"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

var (
	inseeAuthUrl       = "https://api.insee.fr/token"
	inseeCheckUrl      = "https://api.insee.fr/entreprises/sirene/V3/siren/"
	inseeTokenValidity = 604800
	RetryAuth          = 60
	Tracing            = false
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
func NewInsee(authKey string, authSecret string) (i Insee, err error) {
	i = Insee{AuthKey: authKey, AuthSecret: authSecret}
	err = i.SetAuthToken()
	return
}

//NewInseeRefreshed create a refreshed token Insee stuff
func NewInseeRefreshed(authKey string, authSecret string) (i Insee, err error) {
	i, err = NewInsee(authKey, authSecret)
	if err != nil {
		return i, err
	}
	go func() {
		//since we've already called SetAuth in NewInsee, in case of error,
		//avoid calling it again instantly after ...
		time.Sleep(time.Second * time.Duration(RetryAuth))
		err = i.RefreshAuthToken()
	}()
	return
}

//SetAuthToken will set Token from given Key and Secret
func (i *Insee) SetAuthToken() (err error) {
	//no need to refresh token before a while
	//only refresh 12 hours before expiration
	if i.Authed && !i.AuthLastToken.
		Add(time.Duration(inseeTokenValidity)*time.Second).
		Before(time.Now().Add(time.Hour*12)) {
		return
	}
	if i.AuthKey == "" || i.AuthSecret == "" {
		return errors.New("invalid auth key or secret")
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
	if resp.StatusCode != 200 {
		ReportLogTracerError("insee", "SetAuthToken", "Auth failed")
		return errors.New("Server answered : " + resp.Status)
	}

	if err == nil {
		s, _ := ioutil.ReadAll(resp.Body)
		ret := InseeToken{}
		err = json.Unmarshal(s, &ret)
		if err != nil {
			ReportLogTracerError("insee", "SetAuthToken", "JSON decoding problem", err.Error(), string(s))
			return
		}
		i.AuthToken = ret
		inseeTokenValidity = ret.Expires
		i.Authed = i.AuthToken.Token != ""
		if !i.Authed {
			ReportLogTracerError("insee", "SetAuthToken", "Auth token is empty", string(s))
			return errors.New("returned token is empty")
		}
	}
	return
}

//RefreshAuthToken automatically refresh the auth token based on expiry time
func (i *Insee) RefreshAuthToken() (err error) {
	nbRetry := 0
RETRY:
	err = i.SetAuthToken()
	if err != nil {
		ReportLogTracerError("insee", "RefreshAuthToken", "SetAuthToken failed", err.Error())
		if nbRetry < 2 {
			nbRetry++
			time.Sleep(time.Second * time.Duration(RetryAuth)) //wait 1 minute before retry
			goto RETRY
		}
		return
	}
	go func() {
		//by default sirene reply with a token valid for 7 days
		//refreshing every day approximately
		td := time.Duration(inseeTokenValidity/14) * time.Second
		if inseeTokenValidity < 600000 {
			//sleep only One day if less than a week
			td = time.Hour * 12
		}
		time.Sleep(td)
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
	if resp.StatusCode != 200 {
		tmpB, _ := io.ReadAll(resp.Body)
		err = errors.New("server status : " + resp.Status + " body was : " + string(tmpB))
		ReportLogTracerError("insee", "SirenExist", siren, err.Error(), string(tmpB))
		return false
	}
	if err != nil {
		if resp != nil {
			tmpB, _ := io.ReadAll(resp.Body)
			ReportLogTracerError("insee", "SirenExist", siren, err.Error(), string(tmpB))
		} else {
			ReportLogTracerError("insee", "SirenExist", siren, err.Error())
		}
	}
	return err == nil && resp.StatusCode == 200
}

//GetSiren return the API response
func (i *Insee) GetSiren(siren string) (resp SirenBaseResponse, err error) {
	resp = SirenBaseResponse{}
	if !i.Authed || i.AuthToken.Token == "" {
		return resp, errors.New("not authenticated")
	}
	req, _ := http.NewRequest("GET", inseeCheckUrl+siren, nil)
	req.Header.Add("Authorization", i.AuthToken.Type+" "+i.AuthToken.Token)
	client := &http.Client{}
	r, err := client.Do(req)
	if err != nil {
		ReportLogTracerError("insee", "GetSiren", siren, err.Error())
		return
	}
	if r.StatusCode != 200 {
		tmpB, _ := io.ReadAll(r.Body)
		err = errors.New("server status : " + r.Status + " body was : " + string(tmpB))
		ReportLogTracerError("insee", "GetSiren", siren, string(tmpB))
		return
	}
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil { //an error occured ... they are in pure html, no json message ...
		tmpB, _ := io.ReadAll(r.Body)
		err = errors.New("invalid json : " + err.Error() + " body was : " + string(tmpB))
		ReportLogTracerError("insee", "GetSiren", siren, err.Error(), string(tmpB))
	}
	return
}

//GetSirenMultiRequest return a request with multiple parameters
func (i *Insee) GetSirenMultiRequest(query []string) (resp SirenBaseResponses, err error) {
	resp = SirenBaseResponses{}
	if !i.Authed || i.AuthToken.Token == "" {
		return resp, errors.New("not authenticated")
	}

	req, _ := http.NewRequest("GET", inseeCheckUrl+"?q="+strings.Join(query, "&"), nil)
	req.Header.Add("Authorization", i.AuthToken.Type+" "+i.AuthToken.Token)
	client := &http.Client{}
	r, err := client.Do(req)
	if err != nil {
		ReportLogTracerError("insee", "GetSirenMultiRequest", strings.Join(query, "&"), err.Error())
		return
	}
	if r.StatusCode != 200 {
		tmpB, _ := io.ReadAll(r.Body)
		err = errors.New("server status : " + r.Status + " body was : " + string(tmpB))
		ReportLogTracerError("insee", "GetSirenMultiRequest", strings.Join(query, "&"), string(tmpB))
		return
	}
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil { //an error occured ... they are in pure html, no json message ...
		tmpB, _ := io.ReadAll(r.Body)
		err = errors.New("invalid json : " + err.Error() + " body was : " + string(tmpB))
		ReportLogTracerError("insee", "GetSirenMultiRequest", strings.Join(query, "&"), err.Error())
	}
	return
}

//siren get response

//SirenBaseResponse structure for a Sirene API Response
type SirenBaseResponse struct {
	Header    SirenBaseHeader `json:"header"`
	LegalUnit SirenLegalUnit  `json:"uniteLegale"`
}

type SirenBaseResponses struct {
	Header    SirenBaseHeader  `json:"header"`
	LegalUnit []SirenLegalUnit `json:"unitesLegales"`
}

//SirenBaseHeader Header structure for a Sirene API Response
type SirenBaseHeader struct {
	Status  int    `json:"statut"`
	Message string `json:"message"`
}

//SirenLegalUnit return the values of the entitie of a siren API Call
type SirenLegalUnit struct {
	Siren                   string                 `json:"siren"`                             //: "443061841",
	Status                  string                 `json:"statutDiffusionUniteLegale"`        //: "O",
	DateCreationUniteLegale Date                   `json:"dateCreationUniteLegale"`           //: "2002-05-16",
	Sigle                   null.String            `json:"sigleUniteLegale"`                  //: null,
	Sexe                    null.String            `json:"sexeUniteLegale"`                   //: null,
	Prenom1                 null.String            `json:"prenom1UniteLegale"`                //: null,
	Prenom2                 null.String            `json:"prenom2UniteLegale"`                //: null,
	Prenom3                 null.String            `json:"prenom3UniteLegale"`                //: null,
	Prenom4                 null.String            `json:"prenom4UniteLegale"`                //: null,
	Prenom                  null.String            `json:"prenomUsuelUniteLegale"`            //: null,
	Pseudo                  null.String            `json:"pseudonymeUniteLegale"`             //: null,
	IdentifiantAssociation  null.String            `json:"identifiantAssociationUniteLegale"` //: null,
	TrancheEffective        null.String            `json:"trancheEffectifsUniteLegale"`       //: "41",
	AnneeEffectifs          null.String            `json:"anneeEffectifsUniteLegale"`         //: "2018",
	DateDernier             Date                   `json:"dateDernierTraitementUniteLegale"`  //: "2021-07-09T15:09:46",
	NombrePeriodes          int                    `json:"nombrePeriodesUniteLegale"`         //: 10,
	CategorieEntreprise     null.String            `json:"categorieEntreprise"`               //: "ETI",
	AnneeCategorie          null.String            `json:"anneeCategorieEntreprise"`          //: "2018",
	Periods                 []SirenPeriodLegalUnit `json:"periodesUniteLegale"`               //:
}

//SirenPeriodLegalUnit return each periods data from the API
type SirenPeriodLegalUnit struct {
	DateFin                            Date        `json:"dateFin"`                                       //: null,
	DateDebut                          Date        `json:"dateDebut"`                                     //: "2019-01-24",
	EtatAdministratif                  string      `json:"etatAdministratifUniteLegale"`                  //: "A",
	ChangementEtatAdministratif        bool        `json:"changementEtatAdministratifUniteLegale"`        //: false,
	Nom                                null.String `json:"nomUniteLegale"`                                //: null,
	ChangementNom                      bool        `json:"changementNomUniteLegale"`                      //: false,
	NomUsage                           null.String `json:"nomUsageUniteLegale"`                           //: null,
	ChangementNomUsage                 bool        `json:"changementNomUsageUniteLegale"`                 //: false,
	Denomination                       string      `json:"denominationUniteLegale"`                       //: "GOOGLE FRANCE",
	ChangementDenomination             bool        `json:"changementDenominationUniteLegale"`             //: false,
	DenominationUsuelle1               null.String `json:"denominationUsuelle1UniteLegale"`               //: null,
	DenominationUsuelle2               null.String `json:"denominationUsuelle2UniteLegale"`               //: null,
	DenominationUsuelle3               null.String `json:"denominationUsuelle3UniteLegale"`               //: null,
	ChangementDenominationUsuelle      bool        `json:"changementDenominationUsuelleUniteLegale"`      //: false,
	CategorieJuridique                 null.String `json:"categorieJuridiqueUniteLegale"`                 //: "5499",
	ChangementCategorieJuridique       bool        `json:"changementCategorieJuridiqueUniteLegale"`       //: false,
	ActivitePrincipale                 null.String `json:"activitePrincipaleUniteLegale"`                 //: "70.10Z",
	NomenclatureActivitePrincipale     null.String `json:"nomenclatureActivitePrincipaleUniteLegale"`     //: "NAFRev2",
	ChangementActivitePrincipale       bool        `json:"changementActivitePrincipaleUniteLegale"`       //: false,
	NicSiege                           null.String `json:"nicSiegeUniteLegale"`                           //: "00047",
	ChangementNicSiege                 bool        `json:"changementNicSiegeUniteLegale"`                 //: false,
	EconomieSocialeSolidaire           null.String `json:"economieSocialeSolidaireUniteLegale"`           //: "N",
	ChangementEconomieSocialeSolidaire bool        `json:"changementEconomieSocialeSolidaireUniteLegale"` //: true,
	CaractereEmployeur                 null.String `json:"caractereEmployeurUniteLegale"`                 //: "O",
	ChangementCaractereEmployeur       bool        `json:"changementCaractereEmployeurUniteLegale"`       //: false
}

//Some Date format stuff

//Date return the Correct date format for the API
type Date struct {
	time.Time
}

const (
	doLayout1 = "2006-01-02"
	doLayout2 = "2006-01-02T15:04:05"
)

func (ct *Date) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
		ct.Time = time.Time{}
		return
	}
	ct.Time, err = time.Parse(doLayout1, s)
	if err != nil {
		ct.Time, err = time.Parse(doLayout2, s)
	}
	return
}

func (ct *Date) MarshalJSON() ([]byte, error) {
	if ct.Time.UnixNano() == nilTime {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", ct.Time.Format(doLayout2))), nil
}

var nilTime = (time.Time{}).UnixNano()

func (ct *Date) IsSet() bool {
	return ct.UnixNano() != nilTime
}

func ReportLogTracerError(errors ...string) {
	//trace only if allowed
	if !Tracing || !opentracing.IsGlobalTracerRegistered() {
		return
	}
	crash := opentracing.StartSpan("Reporting Insee Error")
	defer crash.Finish()
	for i, e := range errors {
		crash.LogFields(log.String(fmt.Sprintf("error%d", i), e))
	}
	crash.SetTag("error", true)
}
