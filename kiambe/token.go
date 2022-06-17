package kiambe

import (
	"errors"
	"time"

	"git.kanosolution.net/kano/kaos"
	"github.com/ariefdarmawan/kiam/acm"
	"github.com/sebarcode/codekit"
)

type tokenEngine struct {
}

func NewTokenEngine() *tokenEngine {
	te := new(tokenEngine)
	return te
}

func (te *tokenEngine) Claim(ctx *kaos.Context, req codekit.M) (codekit.M, error) {
	res := codekit.M{}
	userid := ctx.Data().Get("jwt-reference-id", "").(string)

	h, e := ctx.DefaultHub()
	if h == nil {
		return res, errors.New("invalid hub")
	}

	tid := req.GetString("TokenID")
	kind := req.GetString("Kind")

	t := new(acm.Token)
	if e = h.GetByID(t, tid); e != nil {
		return res, errors.New("invalid token handler")
	}

	if t.Status == "Claimed" || t.Status == "Expired" {
		return res, errors.New("invalid token handler")
	}

	if t.Kind != kind {
		return res, errors.New("invalid token handler")
	}

	if userid != "" && t.UserID != userid {
		return res, errors.New("invalid token handler")
	}

	if time.Now().After(t.CreateTime.Add(time.Duration(t.ValidDurationInMinute) * time.Minute)) {
		t.Status = "Expired"
		go h.Save(t)
		return res, errors.New("token expired")
	}

	t.Status = "Claimed"
	t.ClaimTime = time.Now()
	go h.Save(t)

	res.Set("TokenID", tid)
	return res, nil
}
