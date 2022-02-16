package goinsee

import (
	"os"
	"testing"
)

func TestInseeAuth(t *testing.T) {
	i, err := NewInsee(os.Getenv("insee_key"), os.Getenv("insee_secret"))
	if err != nil {
		t.Errorf(err.Error())
		return
	}
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
	i, err := NewInsee(os.Getenv("insee_key"), os.Getenv("insee_secret"))
	if err != nil {
		t.Errorf(err.Error())
		return
	}
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

func TestInseeMultiRequestResponse(t *testing.T) {
	i, err := NewInsee(os.Getenv("insee_key"), os.Getenv("insee_secret"))
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	q := []string{"periode(denominationUniteLegale:Google*)"}

	ret, err := i.GetSirenMultiRequest(q)
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

func TestNoKey(t *testing.T) {
	_, err := NewInsee(os.Getenv("insee_key"), os.Getenv("insee_secret")[:10])
	//should fail ...
	if err != nil {
		t.Logf(err.Error())
		return
	}
	t.Errorf("should fail")
}
func TestRetryFailedAuth(t *testing.T) {
	Tracing = true
	RetryAuth = 10
	_, err := NewInseeRefreshed(os.Getenv("insee_key"), os.Getenv("insee_secret")[:10])
	//should fail ...
	if err != nil {
		t.Logf(err.Error())
		return
	}
	t.Errorf("should fail")
}
