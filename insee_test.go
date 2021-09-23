package goinsee

import (
	"os"
	"testing"
)

func TestInseeAuth(t *testing.T) {
	i := NewInsee(os.Getenv("insee_key"), os.Getenv("insee_secret"))
	if i.AuthKey == "" {
		t.Errorf("authKey must be set")
		return
	}
	if i.AuthSecret == "" {
		t.Errorf("authSecret must be set")
		return
	}
	i.RefreshAuthToken()
	if i.AuthToken.Token == "" {
		t.Errorf("authToken must be set")
		return
	}
	if !i.SirenExist("443061841") { //google siren
		t.Errorf("siren check failed")
	}
}

func TestInseeResponse(t *testing.T) {
	i := NewInsee(os.Getenv("insee_key"), os.Getenv("insee_secret"))
	ret, err := i.GetSiren("443061841")
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	if ret.Header.Status != 200 {
		t.Errorf(ret.Header.Message)
		return
	}
	t.Logf("%+v", ret.LegalUnit)
}
