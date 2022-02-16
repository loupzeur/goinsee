package goinsee

import (
	"os"
	"testing"
	"time"
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

func TestRefreshTimer(t *testing.T) {
	date1 := time.Now()
	date2 := time.Now().Add(time.Duration(time.Hour * -1))

	if date1.Before(date2) {
		t.Errorf("date1 should be after date2")
		return
	}

	t.Logf("%+v\n%+v\n%+v\n", time.Duration(inseeTokenValidity/14)*time.Second, date1, date2)

	comp1 := time.Now().Add(time.Duration(inseeTokenValidity) * time.Second)
	t.Logf("%+v\n%+v\n%+v\n", !comp1.Before(time.Now()), comp1, time.Now())
}
